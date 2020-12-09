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

package nesquic

import (
	"fmt"
	"os"

	"github.com/scionproto/scion/go/lib/log"
)

type Logger struct {
	Trace func(msg string, ctx ...interface{})
	Debug func(msg string, ctx ...interface{})
	Info  func(msg string, ctx ...interface{})
	Warn  func(msg string, ctx ...interface{})
	Error func(msg string, ctx ...interface{})
	Crit  func(msg string, ctx ...interface{})
}

var (
	logger *Logger
)

func init() {
	// TODO(matzf) change default to mute
	// By default this library is noisy, to mute it call msquic.MuteLogging
	initLogging(log.Root())
}

// initLogging initializes logging for the nesquic library using the passed scionproto (or similar) logger
func initLogging(baseLogger log.Logger) {
	logger = &Logger{}
	logger.Trace = func(msg string, ctx ...interface{}) { baseLogger.Trace("MSQUIC: "+msg, ctx...) }
	logger.Debug = func(msg string, ctx ...interface{}) { baseLogger.Debug("MSQUIC: "+msg, ctx...) }
	logger.Info = func(msg string, ctx ...interface{}) { baseLogger.Info("MSQUIC: "+msg, ctx...) }
	logger.Warn = func(msg string, ctx ...interface{}) { baseLogger.Warn("MSQUIC: "+msg, ctx...) }
	logger.Error = func(msg string, ctx ...interface{}) { baseLogger.Error("MSQUIC: "+msg, ctx...) }
	logger.Crit = func(msg string, ctx ...interface{}) { baseLogger.Crit("MSQUIC: "+msg, ctx...) }
}

// SetBasicLogging sets nesquic logging to only write to os.Stdout and os.Stderr
func SetBasicLogging() {
	if logger != nil {
		logger.Trace = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stdout, "%v\t%v", msg, ctx) }
		logger.Debug = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stdout, "%v\t%v", msg, ctx) }
		logger.Info = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stdout, "%v\t%v", msg, ctx) }
		logger.Warn = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stdout, "%v\t%v", msg, ctx) }
		logger.Error = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stderr, "%v\t%v", msg, ctx) }
		logger.Crit = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stderr, "%v\t%v", msg, ctx) }
	}
}

// MuteLogging mutes all logging in this library
func MuteLogging() {
	if logger != nil {
		logger.Trace = func(msg string, ctx ...interface{}) {}
		logger.Debug = func(msg string, ctx ...interface{}) {}
		logger.Info = func(msg string, ctx ...interface{}) {}
		logger.Warn = func(msg string, ctx ...interface{}) {}
		logger.Error = func(msg string, ctx ...interface{}) {}
		logger.Crit = func(msg string, ctx ...interface{}) {}
	}
}
