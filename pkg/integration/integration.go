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
	"io"
	"net"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/scionproto/scion/go/lib/snet"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/daemon"
	"github.com/scionproto/scion/go/lib/log"
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
	// ReadySignal should be written to Stdout by the server once it is ready to accept clients.
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
)

// TestMain should be called from the TestMain function in all test packages
// with integration tests.
// This calls Init to parse the supported command flags and load the
// information about the local topology from the environment/files.
func TestMain(m *testing.M) {
	if err := Init(); err != nil {
		log.Error("Failed to Init", "err", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func Init() error {
	_, file, _, _ := runtime.Caller(0)
	projectRoot = path.Join(path.Dir(file), "../..")
	return sintegration.Init(projectRoot)
}

// AppBinPath returns the path to a scion-apps binary built with the projects Makefile.
func AppBinPath(name string) string {
	return path.Join(projectRoot, "bin", name)
}

// AllIAPairs returns all IAPairs that should be tested.
func AllIAPairs() []sintegration.IAPair {
	return sintegration.IAPairs(hostAddr)
}

// DefaultIAPairs returns a small number of relevant IA pairs to be tested.
// In particular, it will return at most one of each of
// - src/dst in same AS, IPv4
// - src/dst in same AS, IPv6
// - src/dst in different AS, IPv4
// - src/dst in different AS, IPv4 to IPv6
// - src/dst in different AS, IPv6 to IPv4
// - src/dst in different AS, IPv6
// Depending on the topology on which these tests are being run, not all might
// be available.
func DefaultIAPairs() []sintegration.IAPair {
	all := sintegration.IAPairs(hostAddr)
	filtered := make([]sintegration.IAPair, 0, 6)

	is4 := func(a *snet.UDPAddr) bool {
		return a.Host.IP.To4() != nil
	}
	is6 := func(a *snet.UDPAddr) bool {
		return !is4(a)
	}

	var hasSame4, hasSame6, has44, has46, has64, has66 bool
	const numCases = 6 // number of hasX cases, for loop break
	for _, p := range all {
		if p.Src.IA == p.Dst.IA {
			if !hasSame4 && is4(p.Src) {
				filtered = append(filtered, p)
				hasSame4 = true
			}
			if !hasSame6 && is6(p.Src) {
				filtered = append(filtered, p)
				hasSame6 = true
			}
		} else {
			if !has44 && is4(p.Src) && is4(p.Dst) {
				filtered = append(filtered, p)
				has44 = true
			}
			if !has46 && is4(p.Src) && is6(p.Dst) {
				filtered = append(filtered, p)
				has46 = true
			}
			if !has64 && is6(p.Src) && is4(p.Dst) {
				filtered = append(filtered, p)
				has46 = true
			}
			if !has66 && is6(p.Src) && is6(p.Dst) {
				filtered = append(filtered, p)
				has66 = true
			}
		}
		if len(filtered) == numCases {
			break
		}
	}
	return filtered
}

func closeAll(closers []io.Closer) error {
	var firstError error
	for _, c := range closers {
		err := c.Close()
		if firstError == nil {
			firstError = err
		}
	}
	return firstError
}

func replacePattern(pattern string, replacement string, args []string) []string {
	ret := make([]string, len(args))
	for i, arg := range args {
		ret[i] = strings.Replace(arg, pattern, replacement, -1)
	}
	return ret
}

// hostAddr gets _a_ host address, the same way appnet does, for a given IA
func hostAddr(ia addr.IA) *snet.UDPAddr {
	daemon, err := getSCIONDAddress(ia)
	if err != nil {
		log.Error("Failed to get sciond address", "err", err)
		return nil
	}
	hostIP, err := defaultLocalIPAddress(daemon)
	if err != nil {
		log.Error("Failed to get valid host IP", "err", err)
		return nil
	}
	return &snet.UDPAddr{IA: ia, Host: &net.UDPAddr{IP: hostIP, Port: 0}}
}

func getSCIONDAddress(ia addr.IA) (addr string, err error) {
	return sintegration.GetSCIONDAddress(sintegration.GenFile(sintegration.SCIONDAddressesFile), ia)
}

func defaultLocalIPAddress(sciondAddress string) (net.IP, error) {
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

func connSciond(ctx context.Context, sciondAddress string) (daemon.Connector, error) {
	sciondConn, err := daemon.NewService(sciondAddress).Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to SCIOND at %s (override with SCION_DAEMON_ADDRESS): %w", sciondAddress, err)
	}
	return sciondConn, nil
}

// findAnyHostInLocalAS returns the IP address of some (infrastructure) host in the local AS.
func findAnyHostInLocalAS(ctx context.Context, sciondConn daemon.Connector) (net.IP, error) {
	bsAddr, err := daemon.TopoQuerier{Connector: sciondConn}.UnderlayAnycast(ctx, addr.SvcCS)
	if err != nil {
		return nil, err
	}
	return bsAddr.IP, nil
}
