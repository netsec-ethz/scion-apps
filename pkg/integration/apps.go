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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/log"
	"github.com/scionproto/scion/pkg/private/serrors"
	"github.com/scionproto/scion/pkg/snet"

	"github.com/netsec-ethz/scion-apps/pkg/integration/sintegration"
)

var _ sintegration.Integration = (*ScionAppsIntegration)(nil)

type ScionAppsIntegration struct {
	clientCmd      string
	serverCmd      string
	clientArgs     []string
	serverArgs     []string
	logDir         string
	ServerOutMatch func(stdout string) error
	ServerErrMatch func(stderrr string) error
	ClientOutMatch func(stdout string) error
	ClientErrMatch func(stderrr string) error
	ClientDelay    time.Duration
	ClientTimeout  time.Duration
}

// NewAppsIntegration returns an implementation of the Integration interface.
// Start{Client|Server} will run the binary program with name and use the given arguments for the client/server.
// Use SrcIAReplace and DstIAReplace in arguments as placeholder for the source and destination IAs.
// When starting a client/server the placeholders will be replaced with the actual values.
// The server should output the ReadySignal to Stdout once it is ready to accept clients.
// If keepLog is true, also store client and server error logs.
func NewAppsIntegration(clientCmd string, serverCmd string, clientArgs, serverArgs []string) *ScionAppsIntegration {
	sai := &ScionAppsIntegration{
		clientCmd:     clientCmd,
		serverCmd:     serverCmd,
		clientArgs:    clientArgs,
		serverArgs:    serverArgs,
		logDir:        "",
		ClientTimeout: DefaultClientTimeout,
		ClientDelay:   0,
	}
	return sai
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
	args = replacePattern(ServerPortReplace, strconv.Itoa(dst.Host.Port), args)
	log.Debug(fmt.Sprintf("Running server command: %v %v\n", sai.serverCmd, strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, sai.serverCmd, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=1", GoIntegrationEnv))
	cmd.Env = append(cmd.Env, fmt.Sprintf("SCION_DAEMON_ADDRESS=%s", sciondAddr))

	id := fmt.Sprintf("server_%s", addr.FormatIA(dst.IA, addr.WithFileSeparator()))
	stdoutLog := sai.openLogFile(id, ".log")
	stderrLog := sai.openLogFile(id, ".err")
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	readyDetector := &detectingWriter{
		Needle: []byte(ReadySignal),
		Signal: make(chan struct{}),
	}
	cmd.Stdout = io.MultiWriter(stdoutLog, stdoutBuf, readyDetector)
	cmd.Stderr = io.MultiWriter(stderrLog, stderrBuf)

	if err = cmd.Start(); err != nil {
		return nil, serrors.WrapStr("Failed to start server", err, "dst", dst.IA)
	}
	select {
	case <-readyDetector.Signal:
		break
	case <-time.After(sintegration.StartServerTimeout):
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, serrors.New("Start server timed out", "dst", dst.IA)
	}

	aw := &appsWaiter{
		id,
		cmd,
		stdoutBuf,
		stderrBuf,
		sai.ServerOutMatch,
		sai.ServerErrMatch,
	}
	return aw, nil
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
	args = replacePattern(ServerPortReplace, strconv.Itoa(dst.Host.Port), args)
	log.Debug(fmt.Sprintf("Running client command: %v %v\n", sai.clientCmd, strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, sai.clientCmd, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=1", GoIntegrationEnv))
	cmd.Env = append(cmd.Env, fmt.Sprintf("SCION_DAEMON_ADDRESS=%s", sciondAddr))

	id := fmt.Sprintf("client_%s", clientID(src, dst))
	stdoutLog := sai.openLogFile(id, ".log")
	stderrLog := sai.openLogFile(id, ".err")
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	cmd.Stdout = io.MultiWriter(stdoutLog, stdoutBuf)
	cmd.Stderr = io.MultiWriter(stderrLog, stderrBuf)

	aw := &appsWaiter{
		id,
		cmd,
		stdoutBuf,
		stderrBuf,
		sai.ClientOutMatch,
		sai.ClientErrMatch,
	}
	return aw, cmd.Start()
}

func (sai *ScionAppsIntegration) initLogDir(name string) error {
	tmpDir := path.Join(os.TempDir(), "scion-apps-integration")
	err := os.MkdirAll(tmpDir, 0777)
	if err != nil {
		log.Error("Failed to create log folder for testrun", "dir", tmpDir, "err", err)
	}
	logDir, err := os.MkdirTemp(tmpDir, strings.ReplaceAll(name, "/", "_"))
	if err != nil {
		log.Error("Failed to create log folder for testrun", "dir", name, "err", err)
		return err
	}
	sai.logDir = logDir
	log.Info(fmt.Sprintf("Log directory: %s", sai.logDir))
	return nil
}

func (sai *ScionAppsIntegration) openLogFile(id, suffix string) *os.File {
	file := path.Join(sai.logDir, id+suffix)
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY|os.O_SYNC, os.FileMode(0644))
	if err != nil {
		panic(err)
	}
	return f
}

// RunTests runs the client and server for each IAPair.
// In case of an error the function is terminated immediately.
func (sai *ScionAppsIntegration) Run(t *testing.T, pairs []sintegration.IAPair) error {
	t.Helper()
	_ = sai.initLogDir(t.Name())
	defer log.Flush()

	// First run all servers
	dsts := sintegration.ExtractUniqueDsts(pairs)
	serverClosers := make([]io.Closer, 0, len(dsts))
	for _, dst := range dsts {
		c, err := sintegration.StartServer(sai, dst)
		if err != nil {
			log.Error(fmt.Sprintf("Error starting server: %s", dst.String()), "err", err)
			_ = closeAll(serverClosers)
			return err
		}
		serverClosers = append(serverClosers, c)
	}
	time.Sleep(sai.ClientDelay)
	// Now start the clients for srcDest pair
	for i, conn := range pairs {
		testInfo := fmt.Sprintf("%v -> %v (%v/%v)", conn.Src.IA, conn.Dst.IA, i+1, len(pairs))
		log.Info(fmt.Sprintf("Test %v: %s", t.Name(), testInfo))
		if err := sintegration.RunClient(sai, conn, sai.ClientTimeout); err != nil {
			_ = closeAll(serverClosers)
			return err
		}
	}
	return closeAll(serverClosers)
}

func clientID(src, dst *snet.UDPAddr) string {
	return fmt.Sprintf("%s_%s", addr.FormatIA(src.IA, addr.WithFileSeparator()),
		addr.FormatIA(dst.IA, addr.WithFileSeparator()))
}

type appsWaiter struct {
	id        string
	cmd       *exec.Cmd
	stdoutBuf *bytes.Buffer
	stderrBuf *bytes.Buffer
	outMatch  func(stdout string) error
	errMatch  func(stderrr string) error
}

func (aw *appsWaiter) Wait() error {
	state, err := aw.cmd.Process.Wait()
	if err != nil {
		return err
	}
	_ = aw.cmd.Wait()
	if state.ExitCode() > 0 { // Ignore servers killed by the framework
		return fmt.Errorf("program %s returned non-zero exit code:\n%s [exit code=%d]\nstdout:\n%s\nstderr:\n%s",
			aw.id, aw.cmd.String(), state.ExitCode(),
			quotedOutput(aw.stdoutBuf.String()),
			quotedOutput(aw.stderrBuf.String()),
		)
	}
	err = aw.checkOutputMatches()
	if err != nil {
		return err
	}
	return nil
}

func (aw *appsWaiter) checkOutputMatches() error {
	if aw.outMatch != nil {
		if err := aw.outMatch(aw.stdoutBuf.String()); err != nil {
			return fmt.Errorf("program %s did not produce the expected standard output: %w Got:\n%s",
				aw.id, err, quotedOutput(aw.stdoutBuf.String()))
		}
	}
	if aw.errMatch != nil {
		if err := aw.outMatch(aw.stderrBuf.String()); err != nil {
			return fmt.Errorf("program %s did not produce the expected error output: %w Got:\n%s",
				aw.id, err, quotedOutput(aw.stderrBuf.String()))
		}
	}
	return nil
}

// detectingWriter is a "black hole" writer that searches the written data for
// a given Needle and closes Signal when found.
type detectingWriter struct {
	Needle []byte
	Signal chan struct{}
	found  bool
	buf    bytes.Buffer
}

func (s *detectingWriter) Write(b []byte) (int, error) {
	if !s.found {
		s.buf.Write(b)
		if bytes.Contains(s.buf.Bytes(), s.Needle) {
			s.found = true
			close(s.Signal)
		}
	}
	return len(b), nil
}

// Sample match functions

func Contains(expected string) func(string) error {
	return func(out string) error {
		if !strings.Contains(out, expected) {
			return fmt.Errorf("does not contain expected string '%s'", expected)
		}
		return nil
	}
}

func RegExp(regularExpression string) func(string) error {
	re := regexp.MustCompile(regularExpression)
	return func(out string) error {
		if !re.MatchString(out) {
			return fmt.Errorf("does not match regexp '%s'", regularExpression)
		}
		return nil
	}
}

func NoPanic(out string) error {
	if strings.Contains(out, "panic: ") {
		return fmt.Errorf("contains unexpected panic marker")
	}
	return nil
}

func quotedOutput(out string) string {
	return "> " + strings.ReplaceAll(out, "\n", "\n> ")
}
