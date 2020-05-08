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
	"regexp"
	"strings"
	"time"

	"github.com/scionproto/scion/go/lib/common"
	sintegration "github.com/scionproto/scion/go/lib/integration"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/serrors"
	"github.com/scionproto/scion/go/lib/snet"
)

var _ sintegration.Integration = (*ScionAppsIntegration)(nil)

type ScionAppsIntegration struct {
	name              string
	cmd               string
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
func NewAppsIntegration(name string, test string, cmd string, clientArgs, serverArgs []string, keepLogs bool) *ScionAppsIntegration {
	log.Info(fmt.Sprintf("Run %s-%s-tests:", name, test))
	sai := &ScionAppsIntegration{
		name:       test,
		cmd:        cmd,
		clientArgs: clientArgs,
		serverArgs: serverArgs,
		logDir:     "",
	}
	if keepLogs {
		sai.initLogDir(name)
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
	log.Debug(fmt.Sprintf("Running server command: %v %v\n", sai.cmd, strings.Join(args, " ")))

	r := &appsWaiter{
		exec.CommandContext(ctx, sai.cmd, args...),
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

	ready := make(chan struct{})
	// parse stdout until we have the ready signal
	// and check the output with serverOutMatchFun.
	go func() {
		defer log.HandlePanic()
		defer sp.Close()
		signal := fmt.Sprintf("%s%s", ReadySignal, dst.IA)
		init := true

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
			log.Debug("Server stdout", "log line", fmt.Sprintf("%s", line))
		}
		if sai.serverOutMatchFun != nil {
			r.stdoutMatch <- stdoutMatch
		} else {
			r.stdoutMatch <- true
		}
	}()

	var logPipeR *io.PipeReader
	var logPipeW *io.PipeWriter
	if sai.logDir != "" {
		// log stderr to file
		logPipeR, logPipeW = io.Pipe()
		go func() {
			sai.writeLog("server", dst.IA.FileFmt(false), dst.IA.FileFmt(false), logPipeR)
		}()
	}

	// Check the stderr with serverErrMatchFun.
	go func() {
		defer log.HandlePanic()
		defer ep.Close()
		defer func() {
			if logPipeW != nil {
				logPipeW.Close()
			}
		}()
		var stderrMatch bool
		scanner := bufio.NewScanner(ep)
		for scanner.Scan() {
			if scanner.Err() != nil {
				log.Error("Error during reading of stderr", "err", scanner.Err())
				return
			}
			line := scanner.Text()
			if sai.serverErrMatchFun != nil {
				stderrMatch = sai.serverErrMatchFun(stderrMatch, line)
			}
			log.Debug("Server stderr", "log line", fmt.Sprintf("%s", line))
			if logPipeW != nil {
				// Propagate to file logger
				fmt.Fprint(logPipeW, fmt.Sprintf("%s\n", line))
			}
		}
		if sai.serverErrMatchFun != nil {
			r.stderrMatch <- stderrMatch
		} else {
			r.stderrMatch <- true
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
	log.Debug(fmt.Sprintf("Running client command: %v %v\n", sai.cmd, strings.Join(args, " ")))

	r := &appsWaiter{
		exec.CommandContext(ctx, sai.cmd, args...),
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
			log.Debug("Client stdout", "msg", line)
		}
		if sai.clientOutMatchFun != nil {
			r.stdoutMatch <- stdoutMatch
		} else {
			r.stdoutMatch <- true
		}
	}()

	var logPipeR *io.PipeReader
	var logPipeW *io.PipeWriter
	if sai.logDir != "" {
		// log stderr to file
		logPipeR, logPipeW = io.Pipe()
		go func() {
			sai.writeLog("client", clientId(src, dst), fmt.Sprintf("%s -> %s", src.IA, dst.IA), logPipeR)
		}()
	}

	// Check the stderr with clientErrMatchFun
	go func() {
		defer func() {
			if logPipeW != nil {
				logPipeW.Close()
			}
		}()
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
			log.Debug("Client stderr", "msg", line)
			if logPipeW != nil {
				// Propagate to file logger
				fmt.Fprint(logPipeW, fmt.Sprintf("%s\n", line))
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
	logDir, err := ioutil.TempDir("", name)
	if err != nil {
		log.Error("Failed to create log folder for testrun", "dir", name, "err", err)
		return err
	}
	sai.logDir = logDir
	log.Info("Log directory:", "path", sai.logDir)
	return nil
}

func (sai *ScionAppsIntegration) writeLog(name, id, startInfo string, ep io.ReadCloser) {
	defer ep.Close()
	f, err := os.OpenFile(fmt.Sprintf("%s/%s_%s.log", sai.logDir, name, id),
		os.O_CREATE|os.O_WRONLY, os.FileMode(0644))
	if err != nil {
		log.Error("Failed to create log file for test run (create)",
			"name", name, "id", id, "err", err)
		return
	}
	defer f.Close()
	// seek to end of file.
	if _, err := f.Seek(0, 2); err != nil {
		log.Error("Failed to create log file for test run (seek)",
			"name", name, "id", id, "err", err)
		return
	}
	w := bufio.NewWriter(f)
	defer w.Flush()
	w.WriteString(sintegration.WithTimestamp(fmt.Sprintf("Starting %s %s\n", name, startInfo)))
	defer func() {
		w.WriteString(sintegration.WithTimestamp(fmt.Sprintf("Finished %s %s\n", name, startInfo)))
	}()
	scanner := bufio.NewScanner(ep)
	for scanner.Scan() {
		w.WriteString(fmt.Sprintf("%s\n", scanner.Text()))
	}
}

func clientId(src, dst *snet.UDPAddr) string {
	return fmt.Sprintf("%s_%s", src.IA.FileFmt(false), dst.IA.FileFmt(false))
}

var _ sintegration.Waiter = (*appsWaiter)(nil)

type appsWaiter struct {
	*exec.Cmd
	stdoutMatch chan bool
	stderrMatch chan bool
}

func (aw *appsWaiter) Wait() (err error) {
	aw.Process.Wait()
	err = checkOutputMatches(aw.stdoutMatch, aw.stderrMatch)
	if err != nil {
		return err
	}
	aw.Cmd.Wait()
	return
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

func Contains(expected string) (func(prev bool, line string) bool) {
	return func(prev bool, line string) bool {
		res := strings.Contains(line, expected)
		return prev || res // return true if any output line contains the string
	}
}

func RegExp(regularExpression string) (func(prev bool, line string) bool) {
	return func(prev bool, line string) bool {
		matched, err := regexp.MatchString(regularExpression, line)
		if err != nil {
			// invalid regexp, don't count as a match
			matched = false
		}
		return prev || matched // return true if any output line matches the expression
	}
}
