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
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
	model "github.com/netsec-ethz/scion-apps/webapp/models"
	. "github.com/netsec-ethz/scion-apps/webapp/util"
)

// results data extraction regex
var reRunTime = `(packet loss, time )(\S*)`
var reRespTime = `(scmp_seq=0 time=)(\S*)`
var rePktLoss = `(\d+)(% packet loss,)`

// ExtractEchoRespData will parse cmd line output from scmp echo for adding EchoItem fields.
func ExtractEchoRespData(resp string, d *model.EchoItem, start time.Time) {
	// store duration in ms
	diff := time.Now().Sub(start)
	d.ActualDuration = int(diff.Nanoseconds() / 1e6)

	// store current epoch in ms
	d.Inserted = time.Now().UnixNano() / 1e6

	log.Info("resp response", "content", resp)

	var data = make(map[string]float32)
	// -1 if no match for response time, indicating response timeout or packets out of order
	data["response_time"] = -1
	var path, err string
	var match bool
	pathNext := false
	r := strings.Split(resp, "\n")
	for i := range r {
		// match response time in unit ms
		match, _ = regexp.MatchString(reRespTime, r[i])
		if match {
			re := regexp.MustCompile(reRespTime)
			tStr := re.FindStringSubmatch(r[i])[2]
			t, _ := time.ParseDuration(tStr)
			data["response_time"] = float32(t.Nanoseconds()) / 1e6
		}

		// match run time
		match, _ = regexp.MatchString(reRunTime, r[i])
		if match {
			re := regexp.MustCompile(reRunTime)
			tStr := re.FindStringSubmatch(r[i])[2]
			t, _ := time.ParseDuration(tStr)
			data["run_time"] = float32(t.Nanoseconds()) / 1e6
		}

		// match packet loss
		match, _ = regexp.MatchString(rePktLoss, r[i])
		if match {
			re := regexp.MustCompile(rePktLoss)
			loss := re.FindStringSubmatch(r[i])[1]
			l, _ := strconv.ParseFloat(loss, 32)
			data["packet_loss"] = float32(l)
		}

		// save used path (default or interactive) for later user display
		if pathNext {
			path = strings.TrimSpace(r[i])
		}
		match, _ = regexp.MatchString(reUPath, r[i])
		pathNext = match

		// evaluate error message potential
		match1, _ := regexp.MatchString(reErr1, r[i])
		match2, _ := regexp.MatchString(reErr2, r[i])
		match3, _ := regexp.MatchString(reErr3, r[i])
		match4, _ := regexp.MatchString(reErr4, r[i])
		match5, _ := regexp.MatchString(reErr5, r[i])

		if match1 {
			re := regexp.MustCompile(reErr1)
			err = re.FindStringSubmatch(r[i])[1]
		} else if match2 {
			re := regexp.MustCompile(reErr2)
			err = re.FindStringSubmatch(r[i])[1]
		} else if match3 {
			re := regexp.MustCompile(reErr3)
			err = re.FindStringSubmatch(r[i])[1]
		} else if match4 {
			re := regexp.MustCompile(reErr4)
			err = re.FindStringSubmatch(r[i])[1]
		} else if match5 {
			re := regexp.MustCompile(reErr5)
			err = re.FindStringSubmatch(r[i])[1]
		}
	}
	log.Info("app response", "data", data)

	d.RunTime, _ = data["run_time"]
	d.ResponseTime, _ = data["response_time"]
	d.PktLoss = int(data["packet_loss"])
	d.Error = err
	d.Path = path
	d.CmdOutput = resp // pipe log output to render in display later
}

// GetEchoByTimeHandler request the echo results stored since provided time.
func GetEchoByTimeHandler(w http.ResponseWriter, r *http.Request, active bool) {
	r.ParseForm()
	since := r.PostFormValue("since")
	log.Info("Requesting echo data since", "timestamp", since)
	// find undisplayed test results
	echoResults, err := model.ReadEchoItemsSince(since)
	if CheckError(err) {
		returnError(w, err)
		return
	}
	log.Debug("Requested data:", "echoResults", echoResults)

	echoJSON, err := json.Marshal(echoResults)
	if CheckError(err) {
		returnError(w, err)
		return
	}
	jsonBuf := []byte(`{ "graph": ` + string(echoJSON))
	json := []byte(`, "active": ` + strconv.FormatBool(active))
	jsonBuf = append(jsonBuf, json...)
	jsonBuf = append(jsonBuf, []byte(`}`)...)

	// ensure % if any, is escaped correctly before writing to printf formatter
	fmt.Fprintf(w, strings.Replace(string(jsonBuf), "%", "%%", -1))
}
