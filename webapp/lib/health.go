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

package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
	. "github.com/netsec-ethz/scion-apps/webapp/util"
)

var defFileHealthCheck = "tests/health/default.json"
var resFileHealthCheck = "data/healthcheck-result.json"
var healthCheckTimeout = time.Duration(60) * time.Second

// DefTests holds the JSON array for all health checks.
type DefTests struct {
	Tests []DefHealthCheck `json:"tests"`
}

// DefHealthCheck holds JSON fields for a health check definition.
type DefHealthCheck struct {
	Label  string `json:"label"`
	Script string `json:"script"`
	Desc   string `json:"desc"`
}

// ResHealthCheck holds JSON fields for a health check result.
type ResHealthCheck struct {
	Label  string `json:"label"`
	Title  string `json:"desc"`
	Reason string `json:"reason"`
	Pass   bool   `json:"pass"`
	Start  int64  `json:"start"`
	End    int64  `json:"end"`
}

// HealthCheckHandler handles calling the default health-check scripts and
// returning the json-formatted results of each script.
func HealthCheckHandler(w http.ResponseWriter, r *http.Request, options *CmdOptions, ia string) {
	hcResFp := path.Join(options.StaticRoot, resFileHealthCheck)
	// read specified tests from json definition
	fp := path.Join(options.StaticRoot, defFileHealthCheck)
	raw, err := ioutil.ReadFile(fp)
	if CheckError(err) {
		fmt.Fprintf(w, `{ "err": "`+err.Error()+`" }`)
		return
	}
	log.Debug("HealthCheckHandler", "resFileHealthCheck", string(raw))

	err = os.Setenv("SCION_ROOT", path.Clean(options.ScionRoot))
	if CheckError(err) {
		fmt.Fprintf(w, `{ "err": "`+err.Error()+`" }`)
		return
	}
	err = os.Setenv("SCION_BIN", path.Clean(options.ScionBin))
	if CheckError(err) {
		fmt.Fprintf(w, `{ "err": "`+err.Error()+`" }`)
		return
	}
	err = os.Setenv("SCION_GEN", path.Clean(options.ScionGen))
	if CheckError(err) {
		fmt.Fprintf(w, `{ "err": "`+err.Error()+`" }`)
		return
	}
	err = os.Setenv("SCION_LOGS", path.Clean(options.ScionLogs))
	if CheckError(err) {
		fmt.Fprintf(w, `{ "err": "`+err.Error()+`" }`)
		return
	}

	var tests DefTests
	err = json.Unmarshal([]byte(raw), &tests)
	if CheckError(err) {
		fmt.Fprintf(w, `{ "err": "`+err.Error()+`" }`)
		return
	}

	// generate local memory struct to export results
	var results []ResHealthCheck
	for _, test := range tests.Tests {
		res := ResHealthCheck{Label: test.Label, Title: test.Desc}
		results = append(results, res)
	}
	// export empty result set first
	jsonRes, err := json.Marshal(results)
	err = ioutil.WriteFile(hcResFp, jsonRes, 0644)
	CheckError(err)

	// execute each script and format results for json
	for i, test := range tests.Tests {
		pass := true
		log.Info(test.Script + ": " + test.Desc)
		// execute script
		cmd := exec.Command("bash", test.Script, ia)
		cmd.Dir = filepath.Dir(fp)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		start := time.Now().UnixNano() / 1e6

		// export results when test starts, timestamp
		results[i].Start = start
		jsonRes, err = json.Marshal(results)
		err = ioutil.WriteFile(hcResFp, jsonRes, 0644)
		CheckError(err)

		// start cmd timeout timer
		go func(idx int) {
			time.Sleep(healthCheckTimeout)
			if results[idx].End == 0 {
				// no match found by timeout, kill, throw err
				log.Error(tests.Tests[idx].Script + " exceeded timeout: " + healthCheckTimeout.String())
				log.Error("Terminating " + tests.Tests[idx].Script + "...")
				err := cmd.Process.Kill()
				CheckError(err)
			}
		}(i)

		err := cmd.Run()
		if CheckError(err) {
			// fail test for non-zero exit code
			pass = false
		}
		end := time.Now().UnixNano() / 1e6
		outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
		if len(outStr) > 0 {
			log.Info(outStr)
		}
		if len(errStr) > 0 {
			// fail test when errors are written to stderr
			pass = false
			log.Error(errStr)
		}
		// format results
		result := strings.Replace((outStr + ` <b>` + errStr + `</b>`), "\n", "<br>", -1)
		result = strings.Replace(result, "\"", "\\\"", -1)

		// export results when test ends, timestamp
		results[i].End = end
		results[i].Pass = pass
		results[i].Reason = result
		jsonRes, err = json.Marshal(results)
		if CheckError(err) {
			fmt.Fprintf(w, `{ "err": %q }`, err.Error())
			return
		}
		log.Debug(string(jsonRes))
		err = ioutil.WriteFile(hcResFp, jsonRes, 0644)
		CheckError(err)
	}

	// ensure all escaped correctly before writing to printf formatter
	fmt.Fprintf(w, string(jsonRes))
}
