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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/serrors"
	"github.com/scionproto/scion/go/lib/snet"

	"github.com/netsec-ethz/scion-apps/pkg/integration/sintegration"
)

var _ sintegration.Integration = (*ScionAppsIntegration)(nil)

type ScionAppsIntegration struct {
	name              string
	clientCmd         string
	serverCmd         string
	clientArgs        []string
	serverArgs        []string
	logDir            string
	serverOutMatchFun func(previous bool, stdout string) bool
	serverErrMatchFun func(previous bool, stderrr string) bool
	clientOutMatchFun func(previous bool, stdout string) bool
	clientErrMatchFun func(previous bool, stderrr string) bool
}

// NewAppsIntegration returns an implementation of the Integration interface.
// Start{Client|Server} will run the binary program with name and use the given arguments for the client/server.
// Use SrcIAReplace and DstIAReplace in arguments as placeholder for the source and destination IAs.
// When starting a client/server the placeholders will be replaced with the actual values.
// The server should output the ReadySignal to Stdout once it is ready to accept clients.
// If keepLog is true, also store client and server error logs.
func NewAppsIntegration(name string, test string, clientCmd string, serverCmd string, clientArgs, serverArgs []string, keepLogs bool) *ScionAppsIntegration {
	log.Info(fmt.Sprintf("Run %s-%s-tests:", name, test))
	sai := &ScionAppsIntegration{
		name:       test,
		clientCmd:  clientCmd,
		serverCmd:  serverCmd,
		clientArgs: clientArgs,
		serverArgs: serverArgs,
		logDir:     "",
	}
	if keepLogs {
		_ = sai.initLogDir(name)
	}
	return sai
}

func (sai *ScionAppsIntegration) Name() string {
	return sai.name
}

// StartServer starts a server and blocks until the ReadySignal is received on Stdout.
func (sai *ScionAppsIntegration) StartServer(ctx context.Context,
	dst *snet.UDPAddr) (sintegration.Waiter, error) {

	sciondAddr, err := getSCIONDAddress(dst.IA)
	if err != nil {
		return nil, serrors.WrapStr("unable to determine SCIOND address", err)
	}
	args := replacePattern(SCIOND, sciondAddr, sai.serverArgs)
	args = replacePattern(DstIAReplace, dst.IA.String(), args)
	args = replacePattern(DstHostReplace, dst.Host.IP.String(), args)
	log.Debug(fmt.Sprintf("Running server command: %v %v\n", sai.serverCmd, strings.Join(args, " ")))

	r := &appsWaiter{
		exec.CommandContext(ctx, sai.serverCmd, args...),
		make(chan bool, 1),
		make(chan bool, 1),
	}

	r.Env = os.Environ()
	r.Env = append(r.Env, fmt.Sprintf("%s=1", GoIntegrationEnv))
	r.Env = append(r.Env, fmt.Sprintf("SCION_DAEMON_ADDRESS=%s", sciondAddr))

	sp, err := r.StdoutPipe()
	if err != nil {
		return nil, err
	}
	ep, err := r.StderrPipe()
	if err != nil {
		return nil, err
	}

	logfile := fmt.Sprintf("server_%s", dst.IA.FileFmt(false))
	startInfo := dst.IA.FileFmt(false)

	ready := make(chan struct{})
	signal := ReadySignal
	init := true
	// parse stdout until we have the ready signal
	// and check the output with serverOutMatchFun.
	sp = sai.pipeLog(logfile+".log", startInfo, sp)
	go func() {
		defer log.HandlePanic()

		var stdoutMatch bool
		scanner := bufio.NewScanner(sp)
		for scanner.Scan() {
			if scanner.Err() != nil {
				log.Error("Error during reading of stdout", "err", scanner.Err())
				return
			}
			line := scanner.Text()
			if init && strings.Contains(line, signal) {
				close(ready)
				init = false
			}
			if sai.serverOutMatchFun != nil {
				stdoutMatch = sai.serverOutMatchFun(stdoutMatch, line)
			}
		}
		if sai.serverOutMatchFun != nil {
			r.stdoutMatch <- stdoutMatch
		} else {
			r.stdoutMatch <- true
		}
	}()

	// Check the stderr with serverErrMatchFun.
	ep = sai.pipeLog(logfile+".err", startInfo, ep)
	go func() {
		defer log.HandlePanic()
		var stderrMatch bool
		scanner := bufio.NewScanner(ep)
		for scanner.Scan() {
			if scanner.Err() != nil {
				log.Error("Error during reading of stderr", "err", scanner.Err())
				return
			}
			line := scanner.Text()
			if init && strings.Contains(line, signal) {
				close(ready)
				init = false
			}
			if sai.serverErrMatchFun != nil {
				stderrMatch = sai.serverErrMatchFun(stderrMatch, line)
			}
		}
		if sai.serverErrMatchFun != nil {
			r.stderrMatch <- stderrMatch
		} else {
			r.stderrMatch <- true
		}
	}()

	if err = r.Start(); err != nil {
		return nil, serrors.WrapStr("Failed to start server", err, "dst", dst.IA)
	}
	select {
	case <-ready:
		return r, err
	case <-time.After(sintegration.StartServerTimeout):
		return nil, serrors.New("Start server timed out", "dst", dst.IA)
	}
}

func (sai *ScionAppsIntegration) StartClient(ctx context.Context,
	src, dst *snet.UDPAddr) (sintegration.Waiter, error) {

	sciondAddr, err := getSCIONDAddress(src.IA)
	if err != nil {
		return nil, serrors.WrapStr("unable to determine SCIOND address", err)
	}
	args := replacePattern(SCIOND, sciondAddr, sai.clientArgs)
	args = replacePattern(SrcIAReplace, src.IA.String(), args)
	args = replacePattern(SrcHostReplace, src.Host.IP.String(), args)
	args = replacePattern(DstIAReplace, dst.IA.String(), args)
	args = replacePattern(DstHostReplace, dst.Host.IP.String(), args)
	log.Debug(fmt.Sprintf("Running client command: %v %v\n", sai.clientCmd, strings.Join(args, " ")))

	r := &appsWaiter{
		exec.CommandContext(ctx, sai.clientCmd, args...),
		make(chan bool, 1),
		make(chan bool, 1),
	}
	r.Env = os.Environ()
	r.Env = append(r.Env, fmt.Sprintf("%s=1", GoIntegrationEnv))
	r.Env = append(r.Env, fmt.Sprintf("SCION_DAEMON_ADDRESS=%s", sciondAddr))

	sp, err := r.StdoutPipe()
	if err != nil {
		return nil, err
	}
	ep, err := r.StderrPipe()
	if err != nil {
		return nil, err
	}

	logfile := fmt.Sprintf("client_%s", clientID(src, dst))
	startInfo := fmt.Sprintf("%s -> %s", src.IA, dst.IA)

	sp = sai.pipeLog(logfile+".log", startInfo, sp)
	// check the output with clientOutMatchFun
	go func() {
		var stdoutMatch bool
		scanner := bufio.NewScanner(sp)
		for scanner.Scan() {
			if scanner.Err() != nil {
				log.Error("Error during reading of stdout", "err", scanner.Err())
			}
			line := scanner.Text()
			if sai.clientOutMatchFun != nil {
				stdoutMatch = sai.clientOutMatchFun(stdoutMatch, line)
			}
		}
		if sai.clientOutMatchFun != nil {
			r.stdoutMatch <- stdoutMatch
		} else {
			r.stdoutMatch <- true
		}
	}()

	// Check the stderr with clientErrMatchFun
	ep = sai.pipeLog(logfile+".err", startInfo, ep)
	go func() {
		var stderrMatch bool
		scanner := bufio.NewScanner(ep)
		for scanner.Scan() {
			if scanner.Err() != nil {
				log.Error("Error during reading of stderr", "err", scanner.Err())
			}
			line := scanner.Text()
			if sai.clientErrMatchFun != nil {
				stderrMatch = sai.clientErrMatchFun(stderrMatch, line)
			}
		}
		if sai.clientErrMatchFun != nil {
			r.stderrMatch <- stderrMatch
		} else {
			r.stderrMatch <- true
		}
	}()

	return r, r.Start()
}

func (sai *ScionAppsIntegration) ServerStdout(outMatch func(bool, string) bool) {
	sai.serverOutMatchFun = outMatch
}

func (sai *ScionAppsIntegration) ServerStderr(errMatch func(bool, string) bool) {
	sai.serverErrMatchFun = errMatch
}

func (sai *ScionAppsIntegration) ClientStdout(outMatch func(bool, string) bool) {
	sai.clientOutMatchFun = outMatch
}

func (sai *ScionAppsIntegration) ClientStderr(errMatch func(bool, string) bool) {
	sai.clientErrMatchFun = errMatch
}

func (sai *ScionAppsIntegration) initLogDir(name string) error {
	tmpDir := path.Join(os.TempDir(), "scion-apps-integration")
	err := os.MkdirAll(tmpDir, 0777)
	if err != nil {
		log.Error("Failed to create log folder for testrun", "dir", tmpDir, "err", err)
	}
	logDir, err := ioutil.TempDir(tmpDir, name)
	if err != nil {
		log.Error("Failed to create log folder for testrun", "dir", name, "err", err)
		return err
	}
	sai.logDir = logDir
	log.Info("Log directory:", "path", sai.logDir)
	return nil
}

func (sai *ScionAppsIntegration) pipeLog(name, startInfo string, r io.ReadCloser) io.ReadCloser {
	if sai.logDir != "" {
		// tee to log
		pipeR, pipeW := io.Pipe()
		tee := io.TeeReader(r, pipeW)
		go func() {
			sai.writeLog(name, startInfo, tee)
			pipeW.Close()
		}()
		return pipeR
	}
	return r
}

func (sai *ScionAppsIntegration) writeLog(name, startInfo string, pipe io.Reader) {
	file := path.Join(sai.logDir, name)
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.FileMode(0644))
	if err != nil {
		log.Error("Failed to create log file for test run (create)",
			"file", file, "err", err)
		return
	}
	defer f.Close()
	_, _ = f.WriteString(sintegration.WithTimestamp(fmt.Sprintf("Starting %s %s\n", name, startInfo)))
	defer func() {
		_, _ = f.WriteString(sintegration.WithTimestamp(fmt.Sprintf("Finished %s %s\n", name, startInfo)))
	}()
	_, _ = io.Copy(f, pipe)
}

func clientID(src, dst *snet.UDPAddr) string {
	return fmt.Sprintf("%s_%s", src.IA.FileFmt(false), dst.IA.FileFmt(false))
}

var _ sintegration.Waiter = (*appsWaiter)(nil)

type appsWaiter struct {
	*exec.Cmd
	stdoutMatch chan bool
	stderrMatch chan bool
}

func (aw *appsWaiter) Wait() error {
	state, err := aw.Process.Wait()
	if err != nil {
		return err
	}
	if state.ExitCode() > 0 { // Ignore servers killed by the framework
		return fmt.Errorf("the program under test returned non-zero exit code:\n%s [exit code=%d]",
			aw.Cmd.String(), state.ExitCode())
	}
	err = checkOutputMatches(aw.stdoutMatch, aw.stderrMatch)
	if err != nil {
		return err
	}
	_ = aw.Cmd.Wait()
	return nil
}

func checkOutputMatches(stdoutRes chan bool, stderrRes chan bool) error {
	result, ok := <-stdoutRes
	if ok {
		if !result {
			return errors.New("the program under test did not produce the expected standard output")
		}
	}
	result, ok = <-stderrRes
	if ok {
		if !result {
			return errors.New("the program under test did not produce the expected error output")
		}
	}
	return nil
}

// Sample match functions

func Contains(expected string) func(prev bool, line string) bool {
	return func(prev bool, line string) bool {
		res := strings.Contains(line, expected)
		return prev || res // return true if any output line contains the string
	}
}

func RegExp(regularExpression string) func(prev bool, line string) bool {
	return func(prev bool, line string) bool {
		matched, err := regexp.MatchString(regularExpression, line)
		if err != nil {
			// invalid regexp, don't count as a match
			matched = false
		}
		return prev || matched // return true if any output line matches the expression
	}
}

func NoPanic() func(prev bool, line string) bool {
	return func(prev bool, line string) bool {
		matched, err := regexp.MatchString("^.*panic: .*$", line)
		if err != nil {
			// invalid regexp, don't count as a match
			return prev
		}
		if init, err := regexp.MatchString("^.*Registered with dispatcher.*$", line); err == nil {
			if init {
				return init
			}
		}
		return prev && !matched
	}
}
