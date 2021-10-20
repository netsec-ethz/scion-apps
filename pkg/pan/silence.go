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

package pan

import (
	"io"
	"log"
	"sync"
)

var logSilencerMutex sync.Mutex
var logSilencerCount int32
var logSilencerOriginal io.Writer

// silenceLog redirects the log.Default writer to a black hole.
// It can be reenabled by calling unsilenceLog.
// These functions can safely be called from multiple goroutines concurrently;
// the log will remain silenced until unsilenceLog was called for each
// silenceLog call.
func silenceLog() {
	logSilencerMutex.Lock()
	defer logSilencerMutex.Unlock()

	logSilencerCount++
	if logSilencerCount == 1 {
		logSilencerOriginal = log.Default().Writer()
		log.Default().SetOutput(io.Discard)
	}
}

func unsilenceLog() {
	logSilencerMutex.Lock()
	defer logSilencerMutex.Unlock()

	logSilencerCount--
	if logSilencerCount == 0 {
		log.Default().SetOutput(logSilencerOriginal)
		logSilencerOriginal = nil
	} else if logSilencerCount < 0 {
		panic("unsilenceLog called more often than silenceLog")
	}
}
