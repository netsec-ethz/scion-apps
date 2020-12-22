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

// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package logger

import (
	"fmt"
	"log"
)

type Logger interface {
	Print(sessionID string, message interface{})
	Printf(sessionID string, format string, v ...interface{})
	PrintCommand(sessionID string, command string, params string)
	PrintResponse(sessionID string, code int, message string)
}

// Use an instance of this to log in a standard format
type StdLogger struct{}

func (logger *StdLogger) Print(sessionID string, message interface{}) {
	log.Printf("%s  %s", sessionID, message)
}

func (logger *StdLogger) Printf(sessionID string, format string, v ...interface{}) {
	logger.Print(sessionID, fmt.Sprintf(format, v...))
}

func (logger *StdLogger) PrintCommand(sessionID string, command string, params string) {
	if command == "PASS" {
		log.Printf("%s > PASS ****", sessionID)
	} else {
		log.Printf("%s > %s %s", sessionID, command, params)
	}
}

func (logger *StdLogger) PrintResponse(sessionID string, code int, message string) {
	log.Printf("%s < %d %s", sessionID, code, message)
}

// Silent logger, produces no output
type DiscardLogger struct{}

func (logger *DiscardLogger) Print(sessionID string, message interface{})                  {}
func (logger *DiscardLogger) Printf(sessionID string, format string, v ...interface{})     {}
func (logger *DiscardLogger) PrintCommand(sessionID string, command string, params string) {}
func (logger *DiscardLogger) PrintResponse(sessionID string, code int, message string)     {}
