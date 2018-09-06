// go run webapp.go -a 0.0.0.0 -p 8080 -r .

package main

import (
    "flag"
    "fmt"
    _ "github.com/mattn/go-sqlite3"
    lib "github.com/perrig/scionlab/webapp/lib"
    model "github.com/perrig/scionlab/webapp/models"
    "io"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "os/exec"
    "path"
    "runtime"
    "strconv"
    "strings"
    "time"
)

var addr = flag.String("a", "0.0.0.0", "server host address")
var port = flag.Int("p", 8080, "server port number")
var root = flag.String("r", ".", "file system path to browse from")
var cmdBufLen = 1024
var browserAddr = "127.0.0.1"
var rootmarker = ".webapp"
var srcpath string

// Ensures an inactive browser will end continuous testing
var maxContTimeout = time.Duration(10) * time.Minute

var bwRequest *http.Request
var bwActive bool
var bwInterval int
var bwTimeKeepAlive time.Time
var bwChanDone = make(chan bool)

func main() {
    flag.Parse()
    _, srcfile, _, _ := runtime.Caller(0)
    srcpath = path.Dir(srcfile)
    // open and manage database
    dbpath := path.Join(srcpath, "webapp.db")
    model.InitDB(dbpath)
    defer model.CloseDB()
    model.CreateBwTestTable()
    go model.MaintainDatabase()
    dataDirPath := path.Join(srcpath, "data")
    if _, err := os.Stat(dataDirPath); os.IsNotExist(err) {
        os.Mkdir(dataDirPath, os.ModePerm)
    }
    // generate client/server default
    lib.GenClientNodeDefaults(srcpath)
    lib.GenServerNodeDefaults(srcpath)
    refreshRootDirectory()
    appsBuildCheck("bwtester")
    appsBuildCheck("camerapp")
    appsBuildCheck("sensorapp")

    http.HandleFunc("/", mainHandler)
    fsStatic := http.FileServer(http.Dir(path.Join(srcpath, "static")))
    http.Handle("/static/", http.StripPrefix("/static/", fsStatic))
    fsImageFetcher := http.FileServer(http.Dir("."))
    http.Handle("/images/", http.StripPrefix("/images/", fsImageFetcher))
    fsFileBrowser := http.FileServer(http.Dir(*root))
    http.Handle("/files/", http.StripPrefix("/files/", fsFileBrowser))

    http.HandleFunc("/command", commandHandler)
    http.HandleFunc("/imglast", findImageHandler)
    http.HandleFunc("/txtlast", findImageInfoHandler)
    http.HandleFunc("/getnodes", getNodesHandler)
    http.HandleFunc("/getbwbytime", getBwByTimeHandler)

    log.Printf("Browser access at http://%s:%d.\n", browserAddr, *port)
    log.Printf("File browser root: %s\n", *root)
    log.Printf("Listening on %s:%d...\n", *addr, *port)
    log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *addr, *port), nil))
}

// Handles loading index.html for user at root.
func mainHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    w.WriteHeader(http.StatusOK)

    filepath := path.Join(srcpath, "template/index.html")
    data, err := ioutil.ReadFile(filepath)
    if err != nil {
        log.Fatal("ioutil.ReadFile() error: " + err.Error())
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Length", fmt.Sprint(len(data)))
    fmt.Fprint(w, string(data))
}

func parseRequest2BwtestItem(r *http.Request, appSel string) *model.BwTestItem {
    d := new(model.BwTestItem)
    d.SIa = r.PostFormValue("ia_ser")
    d.CIa = r.PostFormValue("ia_cli")
    d.SAddr = r.PostFormValue("addr_ser")
    d.CAddr = r.PostFormValue("addr_cli")
    d.SPort, _ = strconv.Atoi(r.PostFormValue("port_ser"))
    d.CPort, _ = strconv.Atoi(r.PostFormValue("port_cli"))
    if appSel == "bwtester" {
        d.CSDuration, _ = strconv.Atoi(r.PostFormValue("dial-cs-sec"))
        d.CSPktSize, _ = strconv.Atoi(r.PostFormValue("dial-cs-size"))
        d.CSPackets, _ = strconv.Atoi(r.PostFormValue("dial-cs-pkt"))
        d.CSBandwidth = d.CSPackets * d.CSPktSize / d.CSDuration * 8
        d.CSDuration = d.CSDuration * 1000 // final storage in ms
        d.SCDuration, _ = strconv.Atoi(r.PostFormValue("dial-sc-sec"))
        d.SCPktSize, _ = strconv.Atoi(r.PostFormValue("dial-sc-size"))
        d.SCPackets, _ = strconv.Atoi(r.PostFormValue("dial-sc-pkt"))
        d.SCBandwidth = d.SCPackets * d.SCPktSize / d.SCDuration * 8
        d.SCDuration = d.SCDuration * 1000 // final storage in ms
    }
    return d
}

func parseBwTest2Cmd(d *model.BwTestItem, appSel string) []string {
    optClient := fmt.Sprintf("-c=%s,[%s]:%d", d.CIa, d.CAddr, d.CPort)
    optServer := fmt.Sprintf("-s=%s,[%s]:%d", d.SIa, d.SAddr, d.SPort)
    binname := getClientLocationBin(appSel)
    command := []string{binname, optServer, optClient}
    if appSel == "bwtester" {
        bwCS := fmt.Sprintf("-cs=%d,%d,%d,%dbps", d.CSDuration/1000, d.CSPktSize,
            d.CSPackets, d.CSBandwidth)
        bwSC := fmt.Sprintf("-sc=%d,%d,%d,%dbps", d.SCDuration/1000, d.SCPktSize,
            d.SCPackets, d.SCBandwidth)
        command = append(command, []string{bwCS, bwSC}...)
    }
    if len(lib.GetLocalIa()) == 0 {
        command = append(command, []string{"-sciondFromIA"}...)
    }
    return command
}

// Handles parsing SCION addresses to execute client app and write results.
func commandHandler(w http.ResponseWriter, r *http.Request) {
    // always parse forms for new/update cmd params
    r.ParseForm()
    appSel := r.PostFormValue("apps")
    continuous, _ := strconv.ParseBool(r.PostFormValue("continuous"))
    interval, _ := strconv.Atoi(r.PostFormValue("interval"))
    if appSel == "" {
        fmt.Fprintf(w, "Unknown SCION client app. Is one selected?")
        return
    }
    if continuous || bwActive {
        // continuous run
        bwTimeKeepAlive = time.Now()
        bwRequest = r
        bwInterval = interval
        if !bwActive {
            // run continuous goroutine
            bwActive = true
            go continuousBwTest()
        } else {
            // continuous goroutine running?
            if continuous {
                // update it
                log.Println("Updating continuous bwtest...")
            } else {
                // end it
                bwActive = false
            }
        }
    } else {
        // single run
        d := parseRequest2BwtestItem(r, appSel)
        command := parseBwTest2Cmd(d, appSel)

        // execute scion go client app with client/server commands
        log.Printf("Executing: %s\n", strings.Join(command, " "))
        cmd := exec.Command(command[0], command[1:]...)

        pipeReader, pipeWriter := io.Pipe()
        cmd.Stdout = pipeWriter
        cmd.Stderr = pipeWriter
        go writeCmdOutput(w, pipeReader, d, appSel)
        cmd.Run()
        pipeWriter.Close()
    }
}

func continuousBwTest() {
    log.Println("Starting continuous bwtest...")
    defer func() {
        log.Println("Ending continuous bwtest...")
    }()
    for bwActive {
        timeUserIdle := time.Since(bwTimeKeepAlive)
        if timeUserIdle > maxContTimeout {
            log.Println("Last browser keep-alive over ", maxContTimeout, " ago")
            bwActive = false
            break
        }
        r := bwRequest
        r.ParseForm()
        appSel := r.PostFormValue("apps")
        d := parseRequest2BwtestItem(r, appSel)
        command := parseBwTest2Cmd(d, appSel)

        log.Printf("Executing: %s\n", strings.Join(command, " "))
        cmd := exec.Command(command[0], command[1:]...)

        pipeReader, pipeWriter := io.Pipe()
        cmd.Stdout = pipeWriter
        cmd.Stderr = pipeWriter

        go writeCmdOutput(nil, pipeReader, d, appSel)
        start := time.Now()
        cmd.Run()
        pipeWriter.Close()
        // block on cmd output finish
        <-bwChanDone
        end := time.Now()
        elapsed := end.Sub(start)
        interval := time.Duration(bwInterval) * time.Second
        // determine sleep interval based on actual test duration
        remaining := time.Duration(0)
        if interval > elapsed {
            remaining = interval - elapsed
        }
        log.Println("Test took", elapsed.Nanoseconds()/1e6,
            "ms, sleeping for remaining interval:", remaining.Nanoseconds()/1e6, "ms")
        time.Sleep(remaining)
    }
}

func appsBuildCheck(app string) {
    binname := getClientLocationBin(app)
    installpath := path.Join(lib.GOPATH, "bin", binname)
    // check for install, and install only if needed
    if _, err := os.Stat(installpath); os.IsNotExist(err) {
        filepath := getClientLocationSrc(app)
        cmd := exec.Command("go", "install")
        cmd.Dir = path.Dir(filepath)
        log.Printf("Installing %s...\n", app)
        cmd.Run()
    } else {
        log.Printf("Existing install, found %s...\n", app)
    }
}

// Parses html selection and returns name of app binary.
func getClientLocationBin(app string) string {
    var binname string
    switch app {
    case "sensorapp":
        binname = "sensorfetcher"
    case "camerapp":
        binname = "imagefetcher"
    case "bwtester":
        binname = "bwtestclient"
    }
    return binname
}

// Parses html selection and returns location of app source.
func getClientLocationSrc(app string) string {
    slroot := "src/github.com/perrig/scionlab"
    var filepath string
    switch app {
    case "sensorapp":
        filepath = path.Join(lib.GOPATH, slroot, "sensorapp/sensorfetcher/sensorfetcher.go")
    case "camerapp":
        filepath = path.Join(lib.GOPATH, slroot, "camerapp/imagefetcher/imagefetcher.go")
    case "bwtester":
        filepath = path.Join(lib.GOPATH, slroot, "bwtester/bwtestclient/bwtestclient.go")
    }
    return filepath
}

// Handles piping command line output to logs, database, and http response writer.
func writeCmdOutput(w http.ResponseWriter, pr *io.PipeReader, d *model.BwTestItem, appSel string) {
    start := time.Now()
    logpath := path.Join(srcpath, "webapp.log")
    file, err := os.Create(logpath)
    if err != nil {
        fmt.Println(err)
    }
    defer func() {
        // monitor end of test here
        go func() { bwChanDone <- true }()
        file.Close()
    }()

    jsonBuf := []byte(``)
    buf := make([]byte, cmdBufLen)
    for {
        n, err := pr.Read(buf)
        if err != nil {
            pr.Close()
            break
        }
        output := buf[0:n]
        jsonBuf = append(jsonBuf, output...)
        // http write response
        if w != nil {
            w.Write(output)
            if f, ok := w.(http.Flusher); ok {
                f.Flush()
            }
        }
        for i := 0; i < n; i++ {
            buf[i] = 0
        }
    }
    if appSel == "bwtester" {
        // parse bwtester data/error
        lib.ExtractBwtestRespData(string(jsonBuf), d, start)
        // store in database
        model.StoreBwTestItem(d)
        lib.WriteBwtestCsv(d, srcpath)
    }
    // log file write response
    nF, err := file.Write(jsonBuf)
    if err != nil {
        fmt.Println(err)
    }
    if nF != len(jsonBuf) {
        fmt.Println("failed to write data")
    }
}

func getBwByTimeHandler(w http.ResponseWriter, r *http.Request) {
    lib.GetBwByTimeHandler(w, r, bwActive, srcpath)
}

// Handles locating most recent image and writing text info data about it.
func findImageInfoHandler(w http.ResponseWriter, r *http.Request) {
    lib.FindImageInfoHandler(w, r)
}

// Handles locating most recent image formatting it for graphic display in response.
func findImageHandler(w http.ResponseWriter, r *http.Request) {
    lib.FindImageHandler(w, r, browserAddr, *port)
}

func getNodesHandler(w http.ResponseWriter, r *http.Request) {
    lib.GetNodesHandler(w, r, srcpath)
}

// Used to workaround cache-control issues by ensuring root specified by user
// has updated last modified date by writing a .webapp file
func refreshRootDirectory() {
    cliFp := path.Join(srcpath, *root, rootmarker)
    err := ioutil.WriteFile(cliFp, []byte(``), 0644)
    if err != nil {
        log.Println("ioutil.WriteFile() error: " + err.Error())
    }
}

// FileBrowseResponseWriter holds modified reponse headers
type FileBrowseResponseWriter struct {
    http.ResponseWriter
}

// WriteHeader prevents caching directory listings based on directory last modified date.
// This is especailly a problem in Chrome, and can serve the browser stale listings.
func (w FileBrowseResponseWriter) WriteHeader(code int) {
    if code == 200 {
        w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate, proxy-revalidate")
    }
    w.ResponseWriter.WriteHeader(code)
}

// Handles custom filtering of file browsing content
func fileBrowseHandler(h http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rw := FileBrowseResponseWriter{w}
        h.ServeHTTP(rw, r)
    })
}
