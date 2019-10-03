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
var reHop = `(\d+)\s+(\S+),\[(\S+)\].+\s(\S+)\s+(\S+)\s+(\S+)`
var reINTF = `(?i:ifid=)(\d+)`

// ExtractTracerouteRespData will parse cmd line output from scmp traceroute for adding TracerouteItem fields.
func ExtractTracerouteRespData(resp string, d *model.TracerouteItem, start time.Time) {
	// store duration in ms
	diff := time.Now().Sub(start)
	d.ActualDuration = int(diff.Nanoseconds() / 1e6)

	// store current epoch in ms
	d.Inserted = time.Now().UnixNano() / 1e6

	//log.Info("resp response", "content", resp)

	var path, err string
	pathNext := false
	r := strings.Split(resp, "\n")
	for i := range r {
		// save used path (default or interactive) for later user display
		if pathNext {
			path = strings.TrimSpace(r[i])
		}
		match, _ := regexp.MatchString(reUPath, r[i])
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

		handleHopData(r[i], d.Inserted)
	}
	log.Debug("***Path: " + path)

	d.Error = err
	d.CmdOutput = resp // pipe log output to render in display later
	d.Path = path
}

// Extract the hop info from traceroute response and store them in the db related to hop
func handleHopData(line string, runTimeKey int64) {
	d := model.TrHopItem{}
	d.RunTimeKey = runTimeKey
	// store current epoch in ms
	d.Inserted = time.Now().UnixNano() / 1e6

	match, _ := regexp.MatchString(reHop, line)
	if match {
		re := regexp.MustCompile(reHop)
		order, _ := strconv.Atoi(re.FindStringSubmatch(line)[1])
		d.Ord = int(order)
		d.HopIa = re.FindStringSubmatch(line)[2]
		d.HopAddr = re.FindStringSubmatch(line)[3]

		matchIntf, _ := regexp.MatchString(reINTF, line)
		if matchIntf {
			reIF := regexp.MustCompile(reINTF)
			intfID, _ := strconv.Atoi(reIF.FindStringSubmatch(line)[1])
			d.IntfID = int(intfID)
		} else {
			d.IntfID = -1
		}

		var RespTime [3]float32
		for i := 0; i < 3; i++ {
			resp := re.FindStringSubmatch(line)[4+i]
			t, err := time.ParseDuration(resp)
			if err == nil {
				RespTime[i] = float32(t.Nanoseconds()) / 1e6
			} else {
				RespTime[i] = -1
			}
		}
		d.RespTime1 = RespTime[0]
		d.RespTime2 = RespTime[1]
		d.RespTime3 = RespTime[2]

		//store hop information in db
		err := model.StoreTrHopItem(&d)
		if err != nil {
			log.Error(fmt.Sprintf("Error storing hop items: %v", err))
		}
	}
}

// GetTracerouteByTimeHandler request the traceroute results stored since provided time.
func GetTracerouteByTimeHandler(w http.ResponseWriter, r *http.Request, active bool) {
	r.ParseForm()
	since := r.PostFormValue("since")
	log.Info("Requesting traceroute data since", "timestamp", since)
	// find undisplayed test results
	tracerouteResults, err := model.ReadTracerouteItemsSince(since)
	if CheckError(err) {
		returnError(w, err)
		return
	}
	// log.Debug("Requested data:", "tracerouteResults", tracerouteResults)

	tracerouteJSON, err := json.Marshal(tracerouteResults)
	if CheckError(err) {
		returnError(w, err)
		return
	}
	jsonBuf := []byte(`{ "graph": ` + string(tracerouteJSON))
	json := []byte(`, "active": ` + strconv.FormatBool(active))
	jsonBuf = append(jsonBuf, json...)
	jsonBuf = append(jsonBuf, []byte(`}`)...)

	//log.Debug(string(jsonBuf))
	// ensure % if any, is escaped correctly before writing to printf formatter
	fmt.Fprintf(w, strings.Replace(string(jsonBuf), "%", "%%", -1))
}
