// go run webapp.go -a 0.0.0.0 -p 8080 -r .

package main

import (
    "flag"
    "fmt"
    "html/template"
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

    _ "github.com/mattn/go-sqlite3"
    lib "github.com/netsec-ethz/scion-apps/webapp/lib"
    model "github.com/netsec-ethz/scion-apps/webapp/models"
)

var addr = flag.String("a", "0.0.0.0", "server host address")
var port = flag.Int("p", 8080, "server port number")
var root = flag.String("r", ".", "file system path to browse from")
var cmdBufLen = 1024
var browserAddr = "127.0.0.1"
var rootmarker = ".webapp"
var srcpath string
var myIa string

// Ensures an inactive browser will end continuous testing
var maxContTimeout = time.Duration(10) * time.Minute

var bwRequest *http.Request
var bwActive bool
var bwInterval int
var bwPathNum string
var bwTimeKeepAlive time.Time
var bwChanDone = make(chan bool)

var templates *template.Template

type Page struct {
    Title string
    MyIA  string
}

func main() {
    flag.Parse()
    _, srcfile, _, _ := runtime.Caller(0)
    srcpath = path.Dir(srcfile)
    // prepare templates
    templates = prepareTemplates(srcpath)
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
    myIa = lib.GetLocalIa()
    refreshRootDirectory()
    appsBuildCheck("bwtester")
    appsBuildCheck("camerapp")
    appsBuildCheck("sensorapp")

    serveExact("/favicon.ico", "./favicon.ico")
    http.HandleFunc("/", mainHandler)
    http.HandleFunc("/about", aboutHandler)
    http.HandleFunc("/apps", appsHandler)
    http.HandleFunc("/astopo", astopoHandler)
    http.HandleFunc("/crt", crtHandler)
    http.HandleFunc("/trc", trcHandler)
    fsStatic := http.FileServer(http.Dir(path.Join(srcpath, "static")))
    http.Handle("/static/", http.StripPrefix("/static/", fsStatic))
    fsImageFetcher := http.FileServer(http.Dir("."))
    http.Handle("/images/", http.StripPrefix("/images/", fsImageFetcher))
    fsFileBrowser := http.FileServer(http.Dir(*root))
    http.Handle("/files/", http.StripPrefix("/files/", fsFileBrowser))

    http.HandleFunc("/command", commandHandler)
    http.HandleFunc("/imglast", findImageHandler)
    http.HandleFunc("/txtlast", lib.FindImageInfoHandler)
    http.HandleFunc("/getnodes", getNodesHandler)
    http.HandleFunc("/getbwbytime", getBwByTimeHandler)
    http.HandleFunc("/healthcheck", healthCheckHandler)
    http.HandleFunc("/dirview", dirViewHandler)

    //ported from scion-viz
    http.HandleFunc("/config", lib.ConfigHandler)
    http.HandleFunc("/labels", lib.LabelsHandler)
    http.HandleFunc("/locations", lib.LocationsHandler)
    http.HandleFunc("/geolocate", lib.GeolocateHandler)
    http.HandleFunc("/getpathtopo", lib.PathTopoHandler)
    http.HandleFunc("/getastopo", lib.AsTopoHandler)
    http.HandleFunc("/getcrt", lib.CrtHandler)
    http.HandleFunc("/gettrc", lib.TrcHandler)

    log.Printf("Browser access at http://%s:%d.\n", browserAddr, *port)
    log.Printf("File browser root: %s\n", *root)
    log.Printf("Listening on %s:%d...\n", *addr, *port)
    log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *addr, *port), nil))
}

func prepareTemplates(srcpath string) *template.Template {
    return template.Must(template.ParseFiles(
        path.Join(srcpath, "template/index.html"),
        path.Join(srcpath, "template/header.html"),
        path.Join(srcpath, "template/footer.html"),
        path.Join(srcpath, "template/files.html"),
        path.Join(srcpath, "template/error.html"),
        path.Join(srcpath, "template/health.html"),
        path.Join(srcpath, "template/about.html"),
        path.Join(srcpath, "template/astopo.html"),
        path.Join(srcpath, "template/crt.html"),
        path.Join(srcpath, "template/trc.html"),
    ))
}

func serveExact(pattern string, filename string) {
    http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, filename)
    })
}

func display(w http.ResponseWriter, tmpl string, data interface{}) {
    templates.ExecuteTemplate(w, tmpl, data)
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        errorHandler(w, r, http.StatusNotFound)
        return
    }
    display(w, "health", &Page{Title: "SCIONLab Health", MyIA: myIa})
}

func errorHandler(w http.ResponseWriter, r *http.Request, status int) {
    w.WriteHeader(status)
    display(w, "error", &Page{Title: "SCIONLab Error", MyIA: myIa})
}

func dirViewHandler(w http.ResponseWriter, r *http.Request) {
    display(w, "dirview", &Page{Title: "SCIONLab Files", MyIA: myIa})
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
    display(w, "about", &Page{Title: "SCIONLab About", MyIA: myIa})
}

func appsHandler(w http.ResponseWriter, r *http.Request) {
    display(w, "apps", &Page{Title: "SCIONLab Apps", MyIA: myIa})
}

func astopoHandler(w http.ResponseWriter, r *http.Request) {
    display(w, "astopo", &Page{Title: "SCIONLab AS Topology", MyIA: myIa})
}

func crtHandler(w http.ResponseWriter, r *http.Request) {
    display(w, "crt", &Page{Title: "SCIONLab Cert", MyIA: myIa})
}

func trcHandler(w http.ResponseWriter, r *http.Request) {
    display(w, "trc", &Page{Title: "SCIONLab TRC", MyIA: myIa})
}

func parseRequest2BwtestItem(r *http.Request, appSel string) (*model.BwTestItem, string) {
    d := new(model.BwTestItem)
    d.SIa = r.PostFormValue("ia_ser")
    d.CIa = r.PostFormValue("ia_cli")
    d.SAddr = r.PostFormValue("addr_ser")
    d.CAddr = r.PostFormValue("addr_cli")
    d.SPort, _ = strconv.Atoi(r.PostFormValue("port_ser"))
    d.CPort, _ = strconv.Atoi(r.PostFormValue("port_cli"))
    addlOpt := r.PostFormValue("addlOpt")
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
    return d, addlOpt
}

func parseBwTest2Cmd(d *model.BwTestItem, appSel string, pathNum string) []string {
    var command []string
    binname := getClientLocationBin(appSel)
    switch appSel {
    case "bwtester", "camerapp", "sensorapp":
        optClient := fmt.Sprintf("-c=%s,[%s]:%d", d.CIa, d.CAddr, d.CPort)
        optServer := fmt.Sprintf("-s=%s,[%s]:%d", d.SIa, d.SAddr, d.SPort)
        log.Printf("optServer %s", optServer)
        command = append(command, binname, optServer, optClient)
        if appSel == "bwtester" {
            bwCS := fmt.Sprintf("-cs=%d,%d,%d,%dbps", d.CSDuration/1000, d.CSPktSize,
                d.CSPackets, d.CSBandwidth)
            bwSC := fmt.Sprintf("-sc=%d,%d,%d,%dbps", d.SCDuration/1000, d.SCPktSize,
                d.SCPackets, d.SCBandwidth)
            command = append(command, bwCS, bwSC)
            if len(pathNum) != 0 {
                // if path choice provided, use interactive mode
                command = append(command, "-i")
            }
        }
    }
    isdCli, _ := strconv.Atoi(strings.Split(d.CIa, "-")[0])
    if isdCli < 16 {
        // -sciondFromIA is better for localhost testing, with test isds
        command = append(command, "-sciondFromIA")
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
    segNum := r.PostFormValue("segNum")
    segType := r.PostFormValue("segType")
    if appSel == "" {
        fmt.Fprintf(w, "Unknown SCION client app. Is one selected?")
        return
    }
    if segType == "PATH" {
        bwPathNum = segNum
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
        d, addlOpt := parseRequest2BwtestItem(r, appSel)
        command := parseBwTest2Cmd(d, appSel, bwPathNum)
        command = append(command, addlOpt)

        // execute scion go client app with client/server commands
        log.Printf("Executing: %s\n", strings.Join(command, " "))
        cmd := exec.Command(command[0], command[1:]...)

        stdin, err := cmd.StdinPipe()
        if err != nil {
            fmt.Println(err)
        }

        pipeReader, pipeWriter := io.Pipe()
        cmd.Stdout = pipeWriter
        cmd.Stderr = pipeWriter

        go writeCmdOutput(w, pipeReader, d, appSel)

        if err = cmd.Start(); err != nil {
            fmt.Println(err)
        }

        in := []byte(bwPathNum + "\n")
        fmt.Println("Writing: " + string(in))
        stdin.Write(in)
        cmd.Wait()

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
        d, addlOpt := parseRequest2BwtestItem(r, appSel)
        command := parseBwTest2Cmd(d, appSel, bwPathNum)
        command = append(command, addlOpt)

        // execute scion go client app with client/server commands
        log.Printf("Executing: %s\n", strings.Join(command, " "))
        cmd := exec.Command(command[0], command[1:]...)

        stdin, err := cmd.StdinPipe()
        if err != nil {
            fmt.Println(err)
        }

        pipeReader, pipeWriter := io.Pipe()
        cmd.Stdout = pipeWriter
        cmd.Stderr = pipeWriter

        go writeCmdOutput(nil, pipeReader, d, appSel)
        start := time.Now()

        if err = cmd.Start(); err != nil {
            fmt.Println(err)
        }

        in := []byte(bwPathNum + "\n")
        fmt.Println("Writing: " + string(in))
        stdin.Write(in)
        cmd.Wait()

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
    case "echo", "traceroute":
        binname = path.Join(lib.GOPATH, lib.SCIONROOT, "bin/scmp")
    case "pingpong":
        binname = path.Join(lib.GOPATH, lib.SCIONROOT, "bin/pingpong")
    }
    return binname
}

// Parses html selection and returns location of app source.
func getClientLocationSrc(app string) string {
    slroot := "src/github.com/netsec-ethz/scion-apps"
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
        fmt.Println(string(output))
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

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
    lib.HealthCheckHandler(w, r, srcpath)
}

func getBwByTimeHandler(w http.ResponseWriter, r *http.Request) {
    lib.GetBwByTimeHandler(w, r, bwActive, srcpath)
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
