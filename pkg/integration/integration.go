// Copyright 2020 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"context"
	"fmt"
	"github.com/scionproto/scion/go/lib/snet"
	"io"
	"net"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet/addrutil"

	"github.com/netsec-ethz/scion-apps/pkg/integration/sintegration"
)

const (
	// SCIOND is a placeholder for the SCIOND server in the arguments.
	SCIOND = "<SCIOND>"
	// ServerPortReplace is a placeholder for the server port in the arguments.
	ServerPortReplace = "<ServerPort>"
	// SrcIAReplace is a placeholder for the source IA in the arguments.
	SrcIAReplace = "<SRCIA>"
	// SrcHostReplace is a placeholder for the source host in the arguments.
	SrcHostReplace = "<SRCHost>"
	// SrcAddrPattern is a placeholder for the source address in the arguments.
	SrcAddrPattern = SrcIAReplace + ",[" + SrcHostReplace + "]"
	// DstIAReplace is a placeholder for the destination IA in the arguments.
	DstIAReplace = "<DSTIA>"
	// DstHostReplace is a placeholder for the destination host in the arguments.
	DstHostReplace = "<DSTHost>"
	// DstAddrPattern is a placeholder for the destination address in the arguments.
	DstAddrPattern = DstIAReplace + ",[" + DstHostReplace + "]"
	// ReadySignal should be written to Stdout by the server once it is read to accept clients.
	// The message should always be `Listening ia=<IA>`
	// where <IA> is the IA the server is listening on.
	ReadySignal = "Listening ia="
	// GoIntegrationEnv is an environment variable that is set for the binary under test.
	// It can be used to guard certain statements, like printing the ReadySignal,
	// in a program under test.
	GoIntegrationEnv = "SCION_GO_INTEGRATION"

	// Default client startup timeout
	DefaultClientTimeout = 10 * time.Second
)

var (
	projectRoot string
	pwd         string
)

// Init initializes the integration test, it adds and validates the command line flags,
// and initializes logging.
func Init(name string) (err error) {
	if pwd, err = os.Getwd(); err != nil {
		return err
	}

	// TODO: make PR to "github.com/scionproto/scion/go/lib/integration"
	// to not rely on hardcoded paths, accept environment variable for gen location
	if SC, ok := os.LookupEnv("SC"); ok {
		if _, err := os.Stat(path.Join(projectRoot, "gen")); os.IsNotExist(err) {
			_ = os.Symlink(path.Join(SC, "gen"), path.Join(projectRoot, "gen"))
		}
	}
	_, file, _, _ := runtime.Caller(0)
	projectRoot = path.Join(path.Dir(file), "../..")
	if _, err := os.Stat("/etc/scion/gen"); err == nil {
		if _, err := os.Stat(path.Join(projectRoot, "gen")); os.IsNotExist(err) {
			_ = os.Symlink("/etc/scion/gen", path.Join(projectRoot, "gen"))
		}
	}

	// Wrap call
	_ = os.Chdir(projectRoot)
	err = sintegration.Init()
	// restore pwd
	_ = os.Chdir(pwd)

	return err
}

// AppBinPath returns the path to a scion-apps binary built with the projects Makefile.
func AppBinPath(name string) string {
	return path.Join(projectRoot, "bin", name)
}

// IAPairs returns all IAPairs that should be tested.
func IAPairs(hostAddr sintegration.HostAddr) []sintegration.IAPair {
	if hostAddr == nil {
		log.Error("Failed to get IAPairs", "err", "hostAddr is nil")
		return []sintegration.IAPair{}
	}
	pairs := sintegration.IAPairs(hostAddr)
	for _, pair := range pairs {
		if pair.Src == nil || pair.Dst == nil {
			log.Error("Invalid IAPairs",
				"err", fmt.Sprintf("IAPair has invalid Src or Dst: %v", pair))
			return []sintegration.IAPair{}
		}
	}
	return pairs
}

// RunTests runs the client and server for each IAPair.
// In case of an error the function is terminated immediately.
func RunTests(in sintegration.Integration, pairs []sintegration.IAPair, clientTimeout time.Duration, clientDelay time.Duration) error {
	defer log.HandlePanic()
	defer log.Flush()
	return sintegration.ExecuteTimed(in.Name(), func() (clossingErr error) {
		// First run all servers
		dsts := sintegration.ExtractUniqueDsts(pairs)
		var serverClosers []io.Closer
		defer func(*[]io.Closer) {
			initialClosingErr := clossingErr
			var firstError error
			for _, c := range serverClosers {
				closerError := c.Close()
				if firstError == nil {
					firstError = closerError
				}
			}
			if initialClosingErr == nil {
				clossingErr = firstError
			}
		}(&serverClosers)
		for _, dst := range dsts {
			c, err := StartServer(in, dst)
			if err != nil {
				log.Error(fmt.Sprintf("Error in server: %s", dst.String()), "err", err)
				return err
			}
			serverClosers = append(serverClosers, c)
		}
		time.Sleep(clientDelay)
		// Now start the clients for srcDest pair
		for i, conn := range pairs {
			testInfo := fmt.Sprintf("%v -> %v (%v/%v)", conn.Src.IA, conn.Dst.IA, i+1, len(pairs))
			log.Info(fmt.Sprintf("Test %v: %s", in.Name(), testInfo))
			if err := sintegration.RunClient(in, conn, clientTimeout); err != nil {
				log.Error(fmt.Sprintf("Error in client: %s", testInfo), "err", err)
				return err
			}
		}
		return nil
	})
}

func replacePattern(pattern string, replacement string, args []string) []string {
	// first copy
	argsCopy := append([]string(nil), args...)
	for i, arg := range argsCopy {
		if strings.Contains(arg, pattern) {
			argsCopy[i] = strings.Replace(arg, pattern, replacement, -1)
		}
	}
	return argsCopy
}

// HostAddr gets _a_ host address, the same way appnet does, for a given IA
var HostAddr sintegration.HostAddr = func(ia addr.IA) *snet.UDPAddr {
	sciond, err := getSCIONDAddress(ia)
	if err != nil {
		log.Error("Failed to get sciond address", "err", err)
		return nil
	}
	hostIP, err := DefaultLocalIPAddress(sciond)
	if err != nil {
		log.Error("Failed to get valid host IP", "err", err)
		return nil
	}
	return &snet.UDPAddr{IA: ia, Host: &net.UDPAddr{IP: hostIP, Port: 0}}
}

func getSCIONDAddress(ia addr.IA) (addr string, err error) {
	// Wrap library call
	_ = os.Chdir(projectRoot)
	addr, err = sintegration.GetSCIONDAddress(sintegration.GenFile(sintegration.SCIONDAddressesFile), ia)
	// restore pwd
	_ = os.Chdir(pwd)

	return
}

func DefaultLocalIPAddress(sciondAddress string) (net.IP, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sciondConn, err := connSciond(ctx, sciondAddress)
	if err != nil {
		return nil, err
	}
	hostInLocalAS, err := findAnyHostInLocalAS(ctx, sciondConn)
	if err != nil {
		return nil, err
	}
	return addrutil.ResolveLocal(hostInLocalAS)
}

func connSciond(ctx context.Context, sciondAddress string) (sciond.Connector, error) {
	sciondConn, err := sciond.NewService(sciondAddress).Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to SCIOND at %s (override with SCION_DAEMON_ADDRESS): %w", sciondAddress, err)
	}
	return sciondConn, nil
}

// findAnyHostInLocalAS returns the IP address of some (infrastructure) host in the local AS.
func findAnyHostInLocalAS(ctx context.Context, sciondConn sciond.Connector) (net.IP, error) {
	bsAddr, err := sciond.TopoQuerier{Connector: sciondConn}.UnderlayAnycast(ctx, addr.SvcCS)
	if err != nil {
		return nil, err
	}
	return bsAddr.IP, nil
}

// Duplicated from "github.com/scionproto/scion/go/lib/integration", but do not swallow error
func (s *serverStop) Close() error {
	s.cancel()
	err := s.wait.Wait()
	return err // Do return the error
}

type serverStop struct {
	cancel context.CancelFunc
	wait   sintegration.Waiter
}

// StartServer runs a server. The server can be stopped by calling Close() on the returned Closer.
// To start a server with a custom context use in.StartServer directly.
func StartServer(in sintegration.Integration, dst *snet.UDPAddr) (io.Closer, error) {
	serverCtx, serverCancel := context.WithCancel(context.Background())
	s, err := in.StartServer(serverCtx, dst)
	if err != nil {
		serverCancel()
		return nil, err
	}
	return &serverStop{serverCancel, s}, nil
}
