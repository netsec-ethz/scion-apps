package lib

import (
    "encoding/csv"
    "encoding/json"
    "fmt"
    model "github.com/perrig/scionlab/webapp/models"
    "io"
    "log"
    "net/http"
    "os"
    "path"
    "regexp"
    "strconv"
    "strings"
    "time"
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

// ExtractBwtestRespData will parse cmd line output from bwtester for adding BwTestItem fields.
func ExtractBwtestRespData(resp string, d *model.BwTestItem, start time.Time) {
    // store duration in ms
    diff := time.Now().Sub(start)
    d.ActualDuration = int(diff.Nanoseconds() / 1e6)

    // store current epoch in ms
    d.Inserted = time.Now().UnixNano() / 1e6

    var data = map[string]map[string]string{}
    var dir, err string
    var match bool
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
        // evaluate error message potential
        match1, _ := regexp.MatchString(reErr1, r[i])
        match2, _ := regexp.MatchString(reErr2, r[i])
        match3, _ := regexp.MatchString(reErr3, r[i])

        if match1 {
            re := regexp.MustCompile(reErr1)
            err = re.FindStringSubmatch(r[i])[1]
        } else if match2 {
            re := regexp.MustCompile(reErr2)
            err = re.FindStringSubmatch(r[i])[1]
        } else if match3 {
            re := regexp.MustCompile(reErr3)
            err = re.FindStringSubmatch(r[i])[1]
        } else if err == "" && r[i] != "" {
            // fallback to first line if err msg needed
            err = r[i]
        }
    }
    fmt.Println("data:", data)
    fmt.Println("err:", err)

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

    if d.CSThroughput == 0 || d.SCThroughput == 0 {
        d.Error = err
    }
}

// GetBwByTimeHandler request the bwtest results stored since provided time.
func GetBwByTimeHandler(w http.ResponseWriter, r *http.Request, active bool, srcpath string) {
    r.ParseForm()
    since := r.PostFormValue("since")
    log.Println("Requesting data since", since)
    // find undisplayed test results
    bwTestResults := model.ReadBwTestItemsSince(since)
    log.Println("Requested data:", bwTestResults)

    bwtestsJSON, err := json.Marshal(bwTestResults)
    if err != nil {
        fmt.Println(err)
        return
    }
    jsonBuf := []byte(`{ "graph": ` + string(bwtestsJSON))
    json := []byte(`, "active": ` + strconv.FormatBool(active))
    jsonBuf = append(jsonBuf, json...)
    // add log results to response if any
    if len(bwTestResults) > 0 {
        logpath := path.Join(srcpath, "webapp.log")
        file, err := os.Open(logpath)
        if err != nil {
            fmt.Println(err)
        }
        defer file.Close()

        json := []byte(`, "log": "`)
        jsonBuf = append(jsonBuf, json...)
        p := make([]byte, logBufLen)
        for {
            n, err := file.Read(p)
            if err == io.EOF {
                break
            }
            // ensure \ if any, is escaped correctly before writing to json value
            jsonBuf = append(jsonBuf, removeOuterQuotes(strconv.QuoteToASCII(string(p[:n])))...)
        }
        jsonBuf = append(jsonBuf, []byte(`"`)...)
    }
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

// WriteBwtestCsv appends the bwtest data in csv-format to srcpath.
func WriteBwtestCsv(bwtest *model.BwTestItem, srcpath string) {
    // newfile name for every day
    dataFileBwtester := "data/bwtester-" + time.Now().Format("2006-01-02") + ".csv"
    bwdataPath := path.Join(srcpath, dataFileBwtester)
    // write headers if file is new
    writeHeader := false
    if _, err := os.Stat(dataFileBwtester); os.IsNotExist(err) {
        writeHeader = true
    }
    // open/create file
    f, err := os.OpenFile(bwdataPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
    if err != nil {
        fmt.Println("Error: ", err)
        return
    }
    w := csv.NewWriter(f)
    // export headers if this is a new file
    if writeHeader {
        headers := bwtest.GetHeaders()
        w.Write(headers)
    }
    values := bwtest.ToSlice()
    w.Write(values)
    w.Flush()
}
