package lib

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os/exec"
    "path"
    "path/filepath"
    "strconv"
    "strings"
)

type Tests struct {
    Tests []Check `json:"tests"`
}
type Check struct {
    Label  string `json:"label"`
    Script string `json:"script"`
    Desc   string `json:"desc"`
}

// HealthCheckHandler handles calling the default health-check scripts and
// returning the json-formatted results of each script.
func HealthCheckHandler(w http.ResponseWriter, r *http.Request, srcpath string) {
    fp := path.Join(srcpath, "tests/health/default.json")
    raw, err := ioutil.ReadFile(fp)
    if err != nil {
        log.Println("ioutil.ReadFile() error: " + err.Error())
    }
    fmt.Println(string(raw))

    var tests Tests
    json.Unmarshal([]byte(raw), &tests)
    // execute each script and format results for json
    jsonBuf := []byte(`{ `)
    for i := 0; i < len(tests.Tests); i++ {
        pass := true
        fmt.Println(tests.Tests[i].Script + ": " + tests.Tests[i].Desc)
        // execute script
        cmd := exec.Command("bash", tests.Tests[i].Script)
        cmd.Dir = filepath.Dir(fp)
        var stdout, stderr bytes.Buffer
        cmd.Stdout = &stdout
        cmd.Stderr = &stderr
        err := cmd.Run()
        if err != nil {
            // fail test for non-zero exit code
            pass = false
            log.Printf("cmd.Run() failed with %s\n", err)
        }
        outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
        if len(outStr) > 0 {
            fmt.Println(outStr)
        }
        if len(errStr) > 0 {
            // fail test when errors are written to stderr
            pass = false
            fmt.Println(errStr)
        }

        // format results
        result := strings.Replace((outStr + ` <b>` + errStr + `</b>`), "\n", "<br>", -1)
        result = strings.Replace(result, "\"", "\\\"", -1)
        json := []byte(`"` + tests.Tests[i].Label + `": {` +
            `"pass":` + strconv.FormatBool(pass) + `,` +
            `"desc":"` + tests.Tests[i].Desc + `",` +
            `"reason":"` + result + `"` +
            `}`)
        jsonBuf = append(jsonBuf, json...)
        if (i + 1) != len(tests.Tests) { // add delimiter
            jsonBuf = append(jsonBuf, []byte(`,`)...)
        }
    }
    jsonBuf = append(jsonBuf, []byte(` }`)...)

    fmt.Printf(string(jsonBuf))

    // ensure all escaped correctly before writing to printf formatter
    fmt.Fprintf(w, string(jsonBuf))
}
