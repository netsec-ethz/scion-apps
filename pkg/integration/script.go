// Copyright 2021 ETH Zurich
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
	"fmt"
	"os"
	"path"
)

// InputPipeScript generates a shell script that runs command with input generated
// by inputCommand. The script will created in tmpDir. Returns the path to the
// generated script. The script will generate more stuff in tmpDir when executed,
// the caller is responsible for cleaning this up. Use `(*testing.T).TempDir()`.
//
// The script is roughly similar to:
//
//	#!/bin/sh
//	inputCommand | command "$@"
//
// The two commands are inserted verbatim (no escaping performed). Be careful
// to wrap longer shell commands into subshells where appropriate.
//
// NOTE: instead of the simple pipe above, we use a named fifo, start the input
// command in the background so that we can *exec* command.
// This helps to ensure that the subprocesses are somewhat reliably cleaned up.
//
// BACKGROUND: when exec.CommandContext kills the process, it sends a SIGKILL
// to only the process itself (not the process group), leaving the subprocesses
// dangling. Due to some additional quirk in the processing of stdout/err in
// exec.Command (go routines processing stdout/err are never stopped,
// https://github.com/golang/go/issues/23019), Wait-ing on the command then
// never returns.
// By using exec in the shell script, we make sure that `command`, instead of
// the parent shell, is the process that will actually be tracked/killed by the
// golang Command. When killing `command`, this should usually also stop the
// `inputCommand` simply by closing the pipe.
//
// NOTE: if this stops working, some alternatives are:
//   - avoid to shell out for the input in the first place and directly write to
//     stdin of the process with a goroutine.
//   - circumvent the stdout/err processing goroutine by using a os.Pipe which
//     can be explicitly closed (as suggested in https://github.com/golang/go/issues/23019)
//   - avoid the CommandContext, which uses Kill, and "cancel" the whole process
//     group explicitly by SIGTERM.
//   - this will NOT work: trap to clean up subprocesses in the shell script,
//     because CommandContext sends SIGKILL. This was a "fun" exercise.
func InputPipeScript(tmpDir, name, inputCommand, command string) string {
	scriptPath := path.Join(tmpDir, fmt.Sprintf("%s_wrapper.sh", name))
	f, err := os.OpenFile(scriptPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0744)
	if err != nil {
		panic(fmt.Sprintf("failed to create temp file: %v", err))
	}
	defer f.Close()

	script := fmt.Sprintf(`#!/bin/sh
fifopath=$(mktemp -d -p "%s")/fifo  # make sub dir to ensure fifo does not exist
mkfifo "$fifopath"
%s > "$fifopath" &
exec %s "$@" < "$fifopath"
`, tmpDir, inputCommand, command)

	_, err = f.WriteString(script)
	if err != nil {
		panic(err)
	}
	return scriptPath
}
