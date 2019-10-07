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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
	model "github.com/netsec-ethz/scion-apps/webapp/models"
	. "github.com/netsec-ethz/scion-apps/webapp/util"
)

var logBufLen = 1024

// results data extraction regex
var reSCHdr = `(?i:s->c results)`
var reCSHdr = `(?i:c->s results)`
var reBwAtt = `(?i:attempted bandwidth:\s*)([0-9.-]*)(?:\s*bps)`
var reBwAch = `(?i:achieved bandwidth:\s*)([0-9.-]*)(?:\s*bps)`
var reItVar = `(?i:interarrival time variance:\s*)([0-9.-]*)(?:\s*ms)`
var reItMin = `(?i:interarrival time min:\s*)([0-9.-]*)(?:\s*ms)`
var reItAvg = `(?i:average interarrival time:\s*)([0-9.-]*)(?:\s*ms)`
var reItMax = `(?i:interarrival time max:\s*)([0-9.-]*)(?:\s*ms)`
var reErr1 = `(?i:err=*)"(.*?)"`
var reErr2 = `(?i:crit msg=*)"(.*?)"`
var reErr3 = `(?i:error:\s*)([\s\S]*)`
var reErr4 = `(?i:eror:\s*)([\s\S]*)`
var reErr5 = `(?i:crit:\s*)([\s\S]*)`
var reUPath = `(?i:using path:)`
var reSPath = `(?i:path=*)"(.*?)"`

// ExtractBwtestRespData will parse cmd line output from bwtester for adding BwTestItem fields.
func ExtractBwtestRespData(resp string, d *model.BwTestItem, start time.Time) {
	// store duration in ms
	diff := time.Now().Sub(start)
	d.ActualDuration = int(diff.Nanoseconds() / 1e6)

	// store current epoch in ms
	d.Inserted = time.Now().UnixNano() / 1e6

	var data = map[string]map[string]string{}
	var dir, path, err string
	var match bool
	pathNext := false
	r := strings.Split(resp, "\n")
	for i := range r {
		match, _ = regexp.MatchString(reCSHdr, r[i])
		if match {
			dir = "cs"
			data[dir] = make(map[string]string)
		}
		match, _ = regexp.MatchString(reSCHdr, r[i])
		if match {
			dir = "sc"
			data[dir] = make(map[string]string)
		}
		match, _ = regexp.MatchString(reBwAtt, r[i])
		if match {
			re := regexp.MustCompile(reBwAtt)
			data[dir]["bandwidth"] = re.FindStringSubmatch(r[i])[1]
		}
		match, _ = regexp.MatchString(reBwAch, r[i])
		if match {
			re := regexp.MustCompile(reBwAch)
			data[dir]["throughput"] = re.FindStringSubmatch(r[i])[1]
		}
		match, _ = regexp.MatchString(reItVar, r[i])
		if match {
			re := regexp.MustCompile(reItVar)
			data[dir]["arrival_var"] = re.FindStringSubmatch(r[i])[1]
		}
		match, _ = regexp.MatchString(reItMin, r[i])
		if match {
			re := regexp.MustCompile(reItMin)
			data[dir]["arrival_min"] = re.FindStringSubmatch(r[i])[1]
		}
		match, _ = regexp.MatchString(reItAvg, r[i])
		if match {
			re := regexp.MustCompile(reItAvg)
			data[dir]["arrival_avg"] = re.FindStringSubmatch(r[i])[1]
		}
		match, _ = regexp.MatchString(reItMax, r[i])
		if match {
			re := regexp.MustCompile(reItMax)
			data[dir]["arrival_max"] = re.FindStringSubmatch(r[i])[1]
		}
		// save used path (default or interactive) for later user display
		match, _ := regexp.MatchString(reSPath, r[i])
		if match {
			re := regexp.MustCompile(reSPath)
			path = re.FindStringSubmatch(r[i])[1]
		} else if pathNext {
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
		} else if err == "" && r[i] != "" {
			// fallback to first line if err msg needed
			err = r[i]
		}
	}
	log.Info("app response", "data", data)

	// get bandwidth from original request
	d.CSThroughput, _ = strconv.Atoi(data["cs"]["throughput"])
	d.CSArrVar, _ = strconv.Atoi(data["cs"]["arrival_var"])
	d.CSArrAvg, _ = strconv.Atoi(data["cs"]["arrival_avg"])
	d.CSArrMin, _ = strconv.Atoi(data["cs"]["arrival_min"])
	d.CSArrMax, _ = strconv.Atoi(data["cs"]["arrival_max"])
	d.SCThroughput, _ = strconv.Atoi(data["sc"]["throughput"])
	d.SCArrVar, _ = strconv.Atoi(data["sc"]["arrival_var"])
	d.SCArrAvg, _ = strconv.Atoi(data["sc"]["arrival_avg"])
	d.SCArrMin, _ = strconv.Atoi(data["sc"]["arrival_min"])
	d.SCArrMax, _ = strconv.Atoi(data["sc"]["arrival_max"])
	d.Path = path

	if d.CSThroughput == 0 || d.SCThroughput == 0 {
		d.Error = err
		log.Error("app error", "err", err)
	}
	d.Log = resp // pipe log output to render in display later
}

// GetBwByTimeHandler request the bwtest results stored since provided time.
func GetBwByTimeHandler(w http.ResponseWriter, r *http.Request, active bool) {
	r.ParseForm()
	since := r.PostFormValue("since")
	log.Info("Requesting bwtest data since", "timestamp", since)
	// find undisplayed test results
	bwTestResults, err := model.ReadBwTestItemsSince(since)
	if CheckError(err) {
		returnError(w, err)
		return
	}
	log.Debug("Requested data:", "bwTestResults", bwTestResults)

	bwtestsJSON, err := json.Marshal(bwTestResults)
	if CheckError(err) {
		returnError(w, err)
		return
	}
	jsonBuf := []byte(`{ "graph": ` + string(bwtestsJSON))
	json := []byte(`, "active": ` + strconv.FormatBool(active))
	jsonBuf = append(jsonBuf, json...)
	jsonBuf = append(jsonBuf, []byte(`}`)...)

	// ensure % if any, is escaped correctly before writing to printf formatter
	fmt.Fprintf(w, strings.Replace(string(jsonBuf), "%", "%%", -1))
}

func removeOuterQuotes(s string) string {
	if len(s) >= 2 {
		if c := s[len(s)-1]; s[0] == c && (c == '"' || c == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// WriteCmdCsv appends the cmd data (bwtest or echo) in csv-format to srcpath.
func WriteCmdCsv(d model.CmdItem, options *CmdOptions, appSel string) {
	// newfile name for every day
	dataFileCmd := "data/" + appSel + "-" + time.Now().Format("2006-01-02") + ".csv"
	cmdDataPath := path.Join(options.StaticRoot, dataFileCmd)
	// write headers if file is new
	writeHeader := false
	if _, err := os.Stat(dataFileCmd); os.IsNotExist(err) {
		writeHeader = true
	}
	// open/create file
	f, err := os.OpenFile(cmdDataPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if CheckError(err) {
		return
	}
	w := csv.NewWriter(f)
	// export headers if this is a new file
	if writeHeader {
		headers := d.GetHeaders()
		w.Write(headers)
	}
	values := d.ToSlice()
	w.Write(values)
	w.Flush()
}
