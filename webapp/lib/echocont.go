package lib

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
	model "github.com/netsec-ethz/scion-apps/webapp/models"
)
// results data extraction regex
var reRespTimeS = `(packet loss, time )(\d*\.?\d*)(s)`
var reRespTimeMs = `(packet loss, time )(\d*\.?\d*)(ms)`
var rePktLoss = `(\d+)(% packet loss,)`

// ExtractEchoRespData will parse cmd line output from scmp echo for adding EchoItem fields.
func ExtractEchoRespData(resp string, d *model.EchoItem) {
	// store current epoch in ms
	d.Inserted = time.Now().UnixNano() / 1e6

	log.Info("resp response", "content", resp)

	var data = make(map[string]string)
	var path, err string
	var match bool
	pathNext := false
	r := strings.Split(resp, "\n")
	for i := range r {
		// match response time in unit s
		match, _ = regexp.MatchString(reRespTimeS, r[i])
		if match {
			re := regexp.MustCompile(reRespTimeS)
			tStr := re.FindStringSubmatch(r[i])[2]
			t, _ := strconv.ParseFloat(tStr, 32)
			tInt := int(t * 1000)
			data["response_time"] = strconv.Itoa(tInt)
		}

		// match response time in unit ms
		match, _ = regexp.MatchString(reRespTimeMs, r[i])
		if match {
			re := regexp.MustCompile(reRespTimeMs)
			tStr := re.FindStringSubmatch(r[i])[2]
			t, _ := strconv.ParseFloat(tStr, 32)
			data["response_time"] = strconv.Itoa(int(t))
		}

		// match packet loss
		match, _ = regexp.MatchString(rePktLoss, r[i])
		if match {
			re := regexp.MustCompile(rePktLoss)
			data["packet_loss"] = re.FindStringSubmatch(r[i])[1] 
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

		if match1 {
			re := regexp.MustCompile(reErr1)
			err = re.FindStringSubmatch(r[i])[1]
			//log.Info("match1", "err", err)
		} else if match2 {
			re := regexp.MustCompile(reErr2)
			err = re.FindStringSubmatch(r[i])[1]
		} else if match3 {
			re := regexp.MustCompile(reErr3)
			err = re.FindStringSubmatch(r[i])[1]
		}
	}
	log.Info("app response", "data", data)

	//log.Info("print parsed result", "error", err)
	//log.Info("print parsed result", "path", path)

	d.ResponseTime, _ = strconv.Atoi(data["response_time"])
	d.PktLoss, _ = strconv.Atoi(data["packet_loss"])
	d.Error = err
	d.Path = path
	d.CmdOutput = resp // pipe log output to render in display later
}