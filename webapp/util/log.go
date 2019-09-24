// Copyright 2019 ETH Zurich
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
// limitations under the License.package main

package logs

import (
	log "github.com/inconshreveable/log15"
)

// CheckError handles Error logging
func CheckError(e error) bool {
	if e != nil {
		logError("Error:", "err", e)
	}
	return e != nil
}

// CheckFatal handles Fatal logging
func CheckFatal(e error) bool {
	if e != nil {
		logFatal("Fatal:", "err", e)
	}
	return e != nil
}

func logError(msg string, a ...interface{}) {
	log.Error(msg, a...)
}

func logFatal(msg string, a ...interface{}) {
	log.Crit(msg, a...)
}
