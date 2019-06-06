// go run webapp.go -a 0.0.0.0 -p 8080 -r .

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/kormat/fmt15"
	_ "github.com/mattn/go-sqlite3"
	lib "github.com/netsec-ethz/scion-apps/webapp/lib"
	model "github.com/netsec-ethz/scion-apps/webapp/models"
	. "github.com/netsec-ethz/scion-apps/webapp/util"
)

// browseRoot is browse-only, consider security (def: cwd)
var browseRoot = flag.String("r", ".",
	"Root path to browse from, CAUTION: read-access granted from -a and -p.")

// staticRoot for serving/writing static data (def: cwd)
var staticRoot = flag.String("s", ".",
	"Static path of web server files (local repo scion-apps/webapp).")

// cwdPath - this is where images are going, this is runtime (record, no settings)
var cwdPath = "."

var addr = flag.String("a", "127.0.0.1", "Address of server host.")
var port = flag.Int("p", 8000, "Port of server host.")
var cmdBufLen = 1024
var browserAddr = "127.0.0.1"
var rootmarker = ".webapp"
var myIa string
var id = "webapp"

// Ensures an inactive browser will end continuous testing
var maxContTimeout = time.Duration(10) * time.Minute

// Continuous cmd case
type contCmd int

const (
	//iota: 0, BwTest: 0
	bwTest contCmd = iota
	echo
)

func (t contCmd) String() string {
	return [...]string{"bwtest", "echo"}[t]
}

var contCmdRequest *http.Request
var contCmdActive bool
var contCmdInterval int
var contCmdTimeKeepAlive time.Time
var contCmdChanDone = make(chan bool)
var pathChoiceTimeout = time.Duration(1000) * time.Millisecond

var templates *template.Template

// Page holds default fields for html template expansion for each page.
type Page struct {
	Title string
	MyIA  string
}

func ensurePath(srcpath, staticDir string) string {
	dir := path.Join(srcpath, staticDir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, os.ModePerm)
	}
	return dir
}

func main() {
	flag.Parse()
	// correct static files are required for the app to serve them, else fail
	if _, err := os.Stat(path.Join(*staticRoot, "static")); os.IsNotExist(err) {
		log.Error("-s flag must be set with local repo: scion-apps/webapp")
		CheckFatal(err)
		return
	}

	// logging
	logDirPath := ensurePath(*staticRoot, "logs")
	log.Root().SetHandler(log.MultiHandler(
		log.LvlFilterHandler(log.LvlDebug,
			log.StreamHandler(os.Stderr, fmt15.Fmt15Format(fmt15.ColorMap))),
		log.LvlFilterHandler(log.LvlInfo,
			log.Must.FileHandler(path.Join(logDirPath, fmt.Sprintf("%s.log", id)),
				fmt15.Fmt15Format(nil)))))
	log.Info("======================> Webapp started")

	// prepare templates
	templates = prepareTemplates(*staticRoot)
	// open and manage database
	dbpath := path.Join(*staticRoot, "webapp.db")
	err := model.InitDB(dbpath)
	if CheckFatal(err) {
		return
	}
	defer model.CloseDB()
	err = model.LoadDB()
	if CheckFatal(err) {
		return
	}
	go model.MaintainDatabase()
	ensurePath(*staticRoot, "data")
	// generate client/server default
	lib.GenClientNodeDefaults(*staticRoot)
	lib.GenServerNodeDefaults(*staticRoot)

	myIa = lib.GetLocalIa()
	if len(myIa) == 0 {
		myIa = lib.GetCliIaDef()
	}
	log.Info("IA loaded:", "myIa", myIa)

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
	fsStatic := http.FileServer(http.Dir(path.Join(*staticRoot, "static")))
	http.Handle("/static/", http.StripPrefix("/static/", fsStatic))
	fsData := http.FileServer(http.Dir(path.Join(*staticRoot, "data")))
	http.Handle("/data/", http.StripPrefix("/data/", fsData))
	fsFileBrowser := http.FileServer(http.Dir(*browseRoot))
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

	log.Info(fmt.Sprintf("Browser access: at http://%s:%d.", browserAddr, *port))
	log.Info("File browser root:", "root", *browseRoot)
	log.Info(fmt.Sprintf("Listening on %s:%d...", *addr, *port))
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", *addr, *port), logRequestHandler(http.DefaultServeMux))
	CheckFatal(err)
}

func logRequestHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info(fmt.Sprintf("%s %s %s", r.RemoteAddr, r.Method, r.URL))
		handler.ServeHTTP(w, r)
	})
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

// There're two CmdItem, BwTestItem and EchoItem
func parseRequest2CmdItem(r *http.Request, appSel string) (model.CmdItem, string) {
	addlOpt := r.PostFormValue("addlOpt")

	if appSel == "echo" { // ###need to be confirmed###
		d := model.EchoItem{}
		d.SIa = r.PostFormValue("ia_ser")
		d.CIa = r.PostFormValue("ia_cli")
		d.SAddr = r.PostFormValue("addr_ser")
		d.CAddr = r.PostFormValue("addr_cli")

		// TODO: parse flags (count, interval, duration from http request)
		// If no flags set then set the default value
		// (to do after finishing the single echo case)
		d.Count = 1
		d.Interval = 1
		d.Timeout = 2

		return d, addlOpt
	}

	d := model.BwTestItem{}
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
	return d, addlOpt
}

// d could be either model.BwTestItem or model.EchoItem
func parseCmdItem2Cmd(dOrinial model.CmdItem, appSel string, pathStr string) []string {
	var command []string
	var isdCli int
	installpath := getClientLocationBin(appSel)

	switch appSel {
	case "bwtester", "camerapp", "sensorapp":
		d, ok := dOrinial.(model.BwTestItem)
		if !ok {
			log.Error("Parsing error, CmdItem category doesn't match its name")
			return nil
		}
		optClient := fmt.Sprintf("-c=%s,[%s]:%d", d.CIa, d.CAddr, d.CPort)
		optServer := fmt.Sprintf("-s=%s,[%s]:%d", d.SIa, d.SAddr, d.SPort)
		command = append(command, installpath, optServer, optClient)
		if appSel == "bwtester" {
			bwCS := fmt.Sprintf("-cs=%d,%d,%d,%dbps", d.CSDuration/1000, d.CSPktSize,
				d.CSPackets, d.CSBandwidth)
			bwSC := fmt.Sprintf("-sc=%d,%d,%d,%dbps", d.SCDuration/1000, d.SCPktSize,
				d.SCPackets, d.SCBandwidth)
			command = append(command, bwCS, bwSC)
			if len(pathStr) > 0 {
				// if path choice provided, use interactive mode
				command = append(command, "-i")
			}
		}
		isdCli, _ = strconv.Atoi(strings.Split(d.CIa, "-")[0])

	case "echo":
		d, ok := dOrinial.(model.EchoItem)
		if !ok {
			fmt.Println("Parsing error, CmdItem category doesn't match its name")
			return nil
		}
		optApp := "echo"
		optLocal := fmt.Sprintf("-local=%s,[%s]", d.CIa, d.CAddr)
		optRemote := fmt.Sprintf("-remote=%s,[%s]", d.SIa, d.SAddr)
		optCount := fmt.Sprintf("-c=%d", d.Count)
		optTimeout := fmt.Sprintf("-timeout=%ds", d.Timeout)
		optInterval := fmt.Sprintf("-interval=%ds", d.Interval)
		// command = append(command, binname, optApp, optRemote, optLocal, optCount)
		command = append(command, installpath, optApp, optRemote, optLocal, optCount, optTimeout, optInterval)
		isdCli, _ = strconv.Atoi(strings.Split(d.CIa, "-")[0])
	}

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
	if appSel == "" {
		fmt.Fprintf(w, "Unknown SCION client app. Is one selected?")
		return
	}
	if continuous || contCmdActive {
		// continuous run bwtest
		contCmdTimeKeepAlive = time.Now()
		contCmdRequest = r
		contCmdInterval = interval
		var t contCmd
		if appSel == "bwtester" {
			t = bwTest
		} else if appSel == "echo" {
			t = echo
		} else {
			log.Error("Cmd type is not valid for continuous case")
			return
		}

		if !contCmdActive {
			// run continuous goroutine
			contCmdActive = true
			go continuousCmd(t)
		} else {
			// continuous goroutine running?
			if continuous {
				// update it
				log.Info(fmt.Sprintf("Updating continuous %s...", t))
			} else {
				// end it
				contCmdActive = false
			}
		}
	} else {
		// single run
		executeCommand(w, r)
	}
}

// Could either be bwtest or echo
func continuousCmd(t contCmd) {
	log.Info(fmt.Sprintf("Starting continuous %s...", t))
	defer func() {
		log.Info(fmt.Sprintf("Ending continuous %s...", t))
	}()
	for contCmdActive {
		timeUserIdle := time.Since(contCmdTimeKeepAlive)
		if timeUserIdle > maxContTimeout {
			log.Warn("Last browser keep-alive expired ", "maxContTimeout", maxContTimeout)
			contCmdActive = false
			break
		}
		r := contCmdRequest
		start := time.Now()
		executeCommand(nil, r)

		// block on cmd output finish
		<-contCmdChanDone
		end := time.Now()
		elapsed := end.Sub(start)
		interval := time.Duration(contCmdInterval) * time.Second
		// determine sleep interval based on actual test duration
		remaining := time.Duration(0)
		if interval > elapsed {
			remaining = interval - elapsed
		}
		log.Info(fmt.Sprintf("Test took %d ms, sleeping for remaining interval: %d ms",
			elapsed.Nanoseconds()/1e6, remaining.Nanoseconds()/1e6))
		time.Sleep(remaining)
	}
}

func executeCommand(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	appSel := r.PostFormValue("apps")
	pathStr := r.PostFormValue("pathStr")
	d, addlOpt := parseRequest2CmdItem(r, appSel)
	command := parseCmdItem2Cmd(d, appSel, pathStr)
	if addlOpt != "" {
		command = append(command, addlOpt)
	}

	// execute scion go client app with client/server commands
	log.Info("Executing:", "command", strings.Join(command, " "))
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = getClientCwd(appSel)

	log.Info("Chosen Path:", "pathStr", pathStr)

	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	CheckError(err)
	stdout, err := cmd.StdoutPipe()
	CheckError(err)
	reader := bufio.NewReader(stdout)

	err = cmd.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start err=%v", err)
		if w != nil {
			w.Write([]byte(err.Error() + "\n"))
		}
	}
	go writeCmdOutput(w, reader, stdin, d, appSel, pathStr, cmd)
	cmd.Wait()
}

func appsBuildCheck(app string) {
	installpath := getClientLocationBin(app)
	if _, err := os.Stat(installpath); os.IsNotExist(err) {
		CheckError(err)
		CheckError(errors.New("App missing, build all apps with 'deps.sh' and 'make'."))
	} else {
		log.Info(fmt.Sprintf("Existing install, found %s...", app))
	}
}

// Parses html selection and returns current working directory for execution.
func getClientCwd(app string) string {
	var cwd string
	switch app {
	case "sensorapp":
		cwd = path.Join(lib.GOPATH, lib.LABROOT, "sensorapp/sensorfetcher")
	case "camerapp":
		cwd = path.Join(lib.GOPATH, lib.LABROOT, "camerapp/imagefetcher")
	case "bwtester":
		cwd = path.Join(lib.GOPATH, lib.LABROOT, "bwtester/bwtestclient")
	case "echo":
		cwd = path.Join(lib.GOPATH, lib.SCIONROOT, "bin")
	}
	return cwd
}

// Parses html selection and returns name of app binary.
func getClientLocationBin(app string) string {
	var binname string
	switch app {
	case "sensorapp":
		binname = path.Join(lib.GOPATH, lib.LABROOT, "sensorapp/sensorfetcher/sensorfetcher")
	case "camerapp":
		binname = path.Join(lib.GOPATH, lib.LABROOT, "camerapp/imagefetcher/imagefetcher")
	case "bwtester":
		binname = path.Join(lib.GOPATH, lib.LABROOT, "bwtester/bwtestclient/bwtestclient")
	case "echo":
		binname = path.Join(lib.GOPATH, lib.SCIONROOT, "bin/scmp")
	}
	return binname
}

// Handles piping command line output to logs, database, and http response writer.
func writeCmdOutput(w http.ResponseWriter, reader io.Reader, stdin io.WriteCloser, d model.CmdItem, appSel string, pathStr string, cmd *exec.Cmd) {
	// regex to find matching path in interactive mode
	var errMsg string
	// reAvailPath := `(?i:\[ *[0-9]*\] hops:)`
	reAvailPath := `(?i:available paths to)`
	rePathStr := `\[(.*?)\].*` + regexp.QuoteMeta(pathStr)
	interactive := len(pathStr) > 0
	if interactive {
		log.Info("Searching:", "regex", rePathStr)
	}
	start := time.Now()

	defer func() {
		// monitor end of test here
		go func() { contCmdChanDone <- true }()
	}()

	pathsAvail := false
	jsonBuf := []byte(``)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		// read each line from stdout
		line := scanner.Text()
		log.Info(line)
		fmt.Fprintln(os.Stdout, line)

		jsonBuf = append(jsonBuf, []byte(line+"\n")...)
		// http write response
		if w != nil {
			w.Write([]byte(line + "\n"))
		}

		if interactive {
			// To prevent indefinite wait for stdin when no match is found, timeout
			match, _ := regexp.MatchString(reAvailPath, line)
			if match {
				pathsAvail = match
				// start stdin wait timer
				go func() {
					time.Sleep(pathChoiceTimeout)
					if pathsAvail {
						// no match found by timeout, kill, throw err
						errMsg = "Path no longer available: " + pathStr
						log.Warn(errMsg)
						log.Info("Terminating app...", "appSel", appSel)
						err := cmd.Process.Kill()
						CheckError(err)
					}
				}()
			}
			// search stdout for matching path
			match, _ = regexp.MatchString(rePathStr, line)
			if match {
				pathsAvail = false
				// write matching number to stdin
				re := regexp.MustCompile(rePathStr)
				num := re.FindStringSubmatch(line)[1]
				pathNum, _ := strconv.Atoi(strings.TrimSpace(num))
				answer := fmt.Sprintf("%d\n", pathNum)
				log.Info("Writing stdin:", "answer", answer)
				stdin.Write([]byte(answer))
			}
		}
	}

	if appSel == "bwtester" {
		// parse bwtester data/error
		d, ok := d.(model.BwTestItem)
		if !ok {
			log.Error("Parsing error, CmdItem category doesn't match its name")
			return
		}
		lib.ExtractBwtestRespData(string(jsonBuf), &d, start)
		if len(errMsg) > 0 {
			d.Error = errMsg
		}
		// store in database
		err := model.StoreBwTestItem(&d)
		if CheckError(err) {
			d.Error = err.Error()
		}
		lib.WriteContCmdCsv(d, *staticRoot, appSel)
	}

	if appSel == "echo" {
		// parse scmp echo data/error
		d, ok := d.(model.EchoItem)
		if !ok {
			log.Error("Parsing error, CmdItem category doesn't match its name")
			return
		}
		lib.ExtractEchoRespData(string(jsonBuf), &d, start)
		if len(errMsg) > 0 {
			d.Error = errMsg
		}
		// store in database
		err := model.StoreEchoItem(&d)
		if CheckError(err) {
			d.Error = err.Error()
		}
		lib.WriteContCmdCsv(d, *staticRoot, appSel)
	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	lib.HealthCheckHandler(w, r, *staticRoot)
}

func getBwByTimeHandler(w http.ResponseWriter, r *http.Request) {
	lib.GetBwByTimeHandler(w, r, contCmdActive, *staticRoot)
}

func getEchoByTimeHandler(w http.ResponseWriter, r *http.Request) {
	lib.GetEchoByTimeHandler(w, r, contCmdActive, *staticRoot)
}

// Handles locating most recent image formatting it for graphic display in response.
func findImageHandler(w http.ResponseWriter, r *http.Request) {
	lib.FindImageHandler(w, r, browserAddr, *port)
}

func getNodesHandler(w http.ResponseWriter, r *http.Request) {
	lib.GetNodesHandler(w, r, *staticRoot)
}

// Used to workaround cache-control issues by ensuring root specified by user
// has updated last modified date by writing a .webapp file
func refreshRootDirectory() {
	cliFp := path.Join(*staticRoot, *browseRoot, rootmarker)
	err := ioutil.WriteFile(cliFp, []byte(``), 0644)
	CheckError(err)
}

// FileBrowseResponseWriter holds modified response headers
type FileBrowseResponseWriter struct {
	http.ResponseWriter
}

// WriteHeader prevents caching directory listings based on directory last modified date.
// This is especially a problem in Chrome, and can serve the browser stale listings.
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
