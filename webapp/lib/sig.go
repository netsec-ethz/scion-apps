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
	"github.com/scionproto/scion/go/lib/addr"
)

var sendFileSigConfig = "tests/sig/sig_send.json"
var recvFileSigConfig = "tests/sig/sig_recv.json"
var resFileSigConfig = "data/sig-result.json"
var sigConfigTimeout = time.Duration(5) * time.Minute

// DefSigTests holds the JSON array for all sig configs.
type DefSigTests struct {
	Tests []DefSigConfig `json:"tests"`
}

// DefSigConfig holds JSON fields for a sig config definition.
type DefSigConfig struct {
	Label  string `json:"label"`
	Script string `json:"script"`
	Desc   string `json:"desc"`
}

// ResSigConfig holds JSON fields for a sig config result.
type ResSigConfig struct {
	Label  string `json:"label"`
	Title  string `json:"desc"`
	Reason string `json:"reason"`
	Pass   bool   `json:"pass"`
	Start  int64  `json:"start"`
	End    int64  `json:"end"`
}

// SigConfigHandler handles calling the default sig-config scripts and
// returning the json-formatted results of each script.
func SigConfigHandler(w http.ResponseWriter, r *http.Request, options *CmdOptions, ia string, sendDirection bool) {

	cIA, _ := addr.IAFromString(ia)
	strIA := strings.Split(cIA.FileFmt(false), "-")
	envvars := []string{
		"IA=" + cIA.FileFmt(false), // local
		"IAd=" + cIA.String(),      // local
		"ISD=" + strIA[0],          // local
		"AS=" + strIA[1],           // local

		"ServePort=" + "8088",    // unused?, fixed
		"IpLocal=" + "10.0.8.A",  // TODO mwfarb get form
		"IpRemote=" + "10.0.8.A", // TODO mwfarb get form

		"SCION_BIN=" + path.Clean(options.ScionBin),
		"SCION_GEN=" + path.Clean(options.ScionGen),
		"SCION_LOGS=" + path.Clean(options.ScionLogs),
		"APPS_ROOT=" + path.Clean(options.AppsRoot),
		"STATIC_ROOT=" + path.Clean(options.StaticRoot),
	}

	IdA := "11"
	IdB := "12"
	CtrlPortA := "80081"
	CtrlPortB := "80083"
	EncapPortA := "80082"
	EncapPortB := "80084"
	if sendDirection {
		envvars = append(envvars, []string{
			"IdLocal=" + IdA,
			"IdRemote=" + IdB,
			"CtrlPortLocal=" + CtrlPortA,
			"CtrlPortRemote=" + CtrlPortB,
			"EncapPortLocal=" + EncapPortA,
			"EncapPortRemote=" + EncapPortB,
		}...)
	} else {
		envvars = append(envvars, []string{
			"IdLocal=" + IdB,
			"IdRemote=" + IdA,
			"CtrlPortLocal=" + CtrlPortB,
			"CtrlPortRemote=" + CtrlPortA,
			"EncapPortLocal=" + EncapPortB,
			"EncapPortRemote=" + EncapPortA,
		}...)
	}

	hcResFp := path.Join(options.StaticRoot, resFileSigConfig)
	// read specified tests from json definition
	var fp string
	if sendDirection {
		fp = path.Join(options.StaticRoot, sendFileSigConfig)
	} else {
		fp = path.Join(options.StaticRoot, recvFileSigConfig)
	}
	raw, err := ioutil.ReadFile(fp)
	if CheckError(err) {
		fmt.Fprintf(w, `{ "err": "`+err.Error()+`" }`)
		return
	}
	log.Debug("SigConfigHandler", "resFileSigConfig", string(raw))

	var tests DefSigTests
	err = json.Unmarshal([]byte(raw), &tests)
	if CheckError(err) {
		fmt.Fprintf(w, `{ "err": "`+err.Error()+`" }`)
		return
	}

	// generate local memory struct to export results
	var results []ResSigConfig
	for _, test := range tests.Tests {
		res := ResSigConfig{Label: test.Label, Title: test.Desc}
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
		cmd.Env = append(os.Environ(), envvars...)
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
			time.Sleep(sigConfigTimeout)
			if results[idx].End == 0 {
				// no match found by timeout, kill, throw err
				log.Error(tests.Tests[idx].Script + " exceeded timeout: " + sigConfigTimeout.String())
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