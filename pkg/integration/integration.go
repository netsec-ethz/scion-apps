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
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/serrors"
	"github.com/scionproto/scion/go/lib/snet"
	sintegration "github.com/scionproto/scion/go/lib/integration"
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
	//ReadySignal = "Listening ia="
	ReadySignal = "Registered with dispatcher\" addr="
	// GoIntegrationEnv is an environment variable that is set for the binary under test.
	// It can be used to guard certain statements, like printing the ReadySignal,
	// in a program under test.
	GoIntegrationEnv = "SCION_GO_INTEGRATION"
	// portString is the string a server prints to specify the port it's listening on.
	portString = "Port="
	// WrapperCmd is the command used to run non-test binaries
	WrapperCmd = "./integration/bin_wrapper.sh"
)

var (
	// FIXME(roosd): The caller to StartServer and StartClient
	// should take care of aggregating the data. I would prefer not to use a
	// global here.
	serverPortsMtx sync.Mutex
	serverPorts    = make(map[addr.IA]string)
)

var _ sintegration.Integration = (*scionAppsIntegration)(nil)

type scionAppsIntegration struct {
	name       string
	cmd        string
	clientArgs []string
	serverArgs []string
	logDir     string
}

// NewAppsIntegration returns an implementation of the Integration interface.
// Start* will run the binary programm with name and use the given arguments for the client/server.
// Use SrcIAReplace and DstIAReplace in arguments as placeholder for the source and destination IAs.
// When starting a client/server the placeholders will be replaced with the actual values.
// The server should output the ReadySignal to Stdout once it is ready to accept clients.
func NewAppsIntegration(name string, cmd string, clientArgs, serverArgs []string, logDir string) sintegration.Integration {
	if logDir != "" {
		logDir = fmt.Sprintf("%s/%s", logDir, name)
		err := os.Mkdir(logDir, os.ModePerm)
		if err != nil && !os.IsExist(err) {
			log.Error("Failed to create log folder for testrun", "dir", name, "err", err)
			return nil
		}
	}
	sai := &scionAppsIntegration{
		name:       name,
		cmd:        cmd,
		clientArgs: clientArgs,
		serverArgs: serverArgs,
		logDir:     logDir,
	}
	return sai
}

func (sai *scionAppsIntegration) Name() string {
	return sai.name
}

// StartServer starts a server and blocks until the ReadySignal is received on Stdout.
func (sai *scionAppsIntegration) StartServer(ctx context.Context, dst *snet.UDPAddr) (sintegration.Waiter, error) {
	args := replacePattern(DstIAReplace, dst.IA.String(), sai.serverArgs)
	args = replacePattern(DstHostReplace, dst.Host.IP.String(), args)

	sciond, err := sintegration.GetSCIONDAddress(sintegration.SCIONDAddressesFile, dst.IA)
	if err != nil {
		return nil, serrors.WrapStr("unable to determine SCIOND address", err)
	}
	args = replacePattern(SCIOND, sciond, args)

	r := &appsWaiter{
		exec.CommandContext(ctx, sai.cmd, args...),
	}
	log.Info(fmt.Sprintf("%v %v\n", sai.cmd, strings.Join(args, " ")))
	r.Env = os.Environ()
	r.Env = append(r.Env, fmt.Sprintf("%s=1", GoIntegrationEnv))
	r.Env = append(r.Env, fmt.Sprintf("SCION_DAEMON_ADDRESS=%s", sciond))
	log.Info("Server info", "sciond", sciond)
	/*ep, err := r.StderrPipe()
	if err != nil {
		return nil, err
	}*/
	sp, err := r.StdoutPipe()
	if err != nil {
		return nil, err
	}
	ready := make(chan struct{})
	// parse until we have the ready signal.
	// and then discard the output until the end (required by StdoutPipe).
	go func() {
		defer log.HandlePanic()
		defer sp.Close()
		signal := fmt.Sprintf("%s%s", ReadySignal, dst.IA)
		init := true
		scanner := bufio.NewScanner(sp)
		for scanner.Scan() {
			if scanner.Err() != nil {
				log.Error("Error during reading of stdout", "err", scanner.Err())
				return
			}
			line := scanner.Text()
			if strings.HasPrefix(line, portString) {
				serverPortsMtx.Lock()
				serverPorts[dst.IA] = strings.TrimPrefix(line, portString)
				serverPortsMtx.Unlock()
			}
			if init && strings.Contains(line, signal) {
				close(ready)
				init = false
			}
			log.Info("Server stdout", "log line", fmt.Sprintf("%s", line))
		}
	}()

	if err = r.Start(); err != nil {
		return nil, common.NewBasicError("Failed to start server", err, "dst", dst.IA)
	}
	select {
	case <-ready:
		return r, err
	case <-time.After(sintegration.StartServerTimeout):
		return nil, common.NewBasicError("Start server timed out", nil, "dst", dst.IA)
	}
}

func (sai *scionAppsIntegration) StartClient(ctx context.Context,
	src, dst *snet.UDPAddr) (sintegration.Waiter, error) {

	args := replacePattern(SrcIAReplace, src.IA.String(), sai.clientArgs)
	args = replacePattern(SrcHostReplace, src.Host.IP.String(), args)
	args = replacePattern(DstIAReplace, dst.IA.String(), args)
	args = replacePattern(DstHostReplace, dst.Host.IP.String(), args)
	args = replacePattern(ServerPortReplace, serverPorts[dst.IA], args)

	sciond, err := sintegration.GetSCIONDAddress(sintegration.SCIONDAddressesFile, src.IA)
	if err != nil {
		return nil, serrors.WrapStr("unable to determine SCIOND address", err)
	}
	args = replacePattern(SCIOND, sciond, args)

	r := &appsWaiter{
		exec.CommandContext(ctx, sai.cmd, args...),
	}
	log.Info(fmt.Sprintf("%v %v\n", sai.cmd, strings.Join(args, " ")))
	r.Env = os.Environ()
	r.Env = append(r.Env, fmt.Sprintf("%s=1", GoIntegrationEnv))
	r.Env = append(r.Env, fmt.Sprintf("SCION_DAEMON_ADDRESS=%s", sciond))
	log.Info("Client info", "sciond", sciond)
	/*ep, err := r.StderrPipe()
	if err != nil {
		return nil, err
	}*/
	sp, err := r.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if sai.logDir != "" {
		fmt.Println("Log directory:", sai.logDir)
	}

	go func() {
		scanner := bufio.NewScanner(sp)
		for scanner.Scan() {
			if scanner.Err() != nil {
				log.Error("Error during reading of stdout", "err", scanner.Err())
			}
			line := scanner.Text()
			log.Info("Server stdout", "msg", line)
		}
	}()

	return r, r.Start()
}

// RunTests runs the client and server for each IAPair.
// In case of an error the function is terminated immediately.
func RunTests(in sintegration.Integration, pairs []sintegration.IAPair, clientTimeout time.Duration) error {
	defer log.HandlePanic()
	defer log.Flush()
	return sintegration.ExecuteTimed(in.Name(), func() error {
		// First run all servers
		dsts := sintegration.ExtractUniqueDsts(pairs)
		for _, dst := range dsts {
			c, err := sintegration.StartServer(in, dst)
			if err != nil {
				log.Error(fmt.Sprintf("Error in server: %s", dst.String()), "err", err)
				return err
			} else {
				defer c.Close()
			}
		}
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

func clientId(src, dst *snet.UDPAddr) string {
	return fmt.Sprintf("%s_%s", src.IA.FileFmt(false), dst.IA.FileFmt(false))
}

var _ sintegration.Waiter = (*appsWaiter)(nil)

type appsWaiter struct {
	*exec.Cmd
}


// Init initializes the integration test, it adds and validates the command line flags,
// and initializes logging.
func Init(name string) error {
	return sintegration.Init(name)
}

// IAPairs returns all IAPairs that should be tested.
func IAPairs(hostAddr sintegration.HostAddr) []sintegration.IAPair {
	return sintegration.IAPairs(hostAddr)
}

// DispAddr reads the CS host Addr from the topology for the specified IA. In general this
// could be the IP of any service (PS/BS/CS) in that IA because they share the same dispatcher in
// the dockerized topology.
// The host IP is used as client or server address in the tests because the testing container is
// connecting to the dispatcher of the services.
var DispAddr sintegration.HostAddr = sintegration.DispAddr

