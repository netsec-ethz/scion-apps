// Copyright 2018 ETH Zurich
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

// NOTE: Webapp relies on SCION's configuration for some of its functionality.
// If the topology changes, webapp should be restarted as well.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
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

// GOPATH is the root of the GOPATH environment (in development).
var GOPATH = os.Getenv("GOPATH")

// browseRoot is browse-only, consider security (def: cwd)
var browseRoot = flag.String("r", ".",
	"Root path to read/browse from, CAUTION: read-access granted from -a and -p.")

// staticRoot for serving/writing static data
var staticRoot = flag.String("srvroot", path.Join(GOPATH, "src/github.com/netsec-ethz/scion-apps/webapp/web"),
	"Path to read/write web server files.")

// appsRoot is the root location of scionlab apps.
var appsRoot = flag.String("sabin", path.Join(GOPATH, "bin"),
	"Path to execute the installed scionlab apps binaries")

// scionRoot is the root location of the scion infrastructure.
var scionRoot = flag.String("sroot", path.Join(GOPATH, "src/github.com/scionproto/scion"),
	"Path to read SCION root directory of infrastructure")
var scionBin = flag.String("sbin", path.Join(*scionRoot, "bin"),
	"Path to execute SCION bin directory of infrastructure tools")
var scionGen = flag.String("sgen", path.Join(*scionRoot, "gen"),
	"Path to read SCION gen directory of infrastructure config")
var scionGenCache = flag.String("sgenc", path.Join(*scionRoot, "gen-cache"),
	"Path to read SCION gen-cache directory of infrastructure run-time config")
var scionLogs = flag.String("slogs", path.Join(*scionRoot, "logs"),
	"Path to read SCION logs directory of infrastructure logging")

var addr = flag.String("a", "127.0.0.1", "Address of server host.")
var port = flag.Int("p", 8000, "Port of server host.")
var cmdBufLen = 1024
var browserAddr = "127.0.0.1"
var settings lib.UserSetting
var id = "webapp"

const reAvailPath = `(?i:available paths to)`
const reRemoveAnsi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var localIAs []string

// Ensures an inactive browser will end continuous testing
var maxContTimeout = time.Duration(10) * time.Minute

// Enum for Continuous cmd case: bwtest and echo
type contCmd int

const (
	//iota: 0, BwTest: 0, Echo: 1, Traceroute: 2
	bwTest contCmd = iota
	echo
	traceroute
)

func (t contCmd) String() string {
	return [...]string{"bwtest", "echo", "traceroute"}[t]
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

var options lib.CmdOptions

func ensurePath(srcpath, staticDir string) string {
	dir := path.Join(srcpath, staticDir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, os.ModePerm)
	}
	return dir
}
func checkPath(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		CheckError(err)
	}
}

func main() {
	flag.Parse()
	options = lib.CmdOptions{*staticRoot, *browseRoot, *appsRoot, *scionRoot, *scionBin, *scionGen, *scionGenCache, *scionLogs}
	// correct static files are required for the app to serve them, else fail
	if _, err := os.Stat(path.Join(options.StaticRoot, "static")); os.IsNotExist(err) {
		log.Error("-s flag must be set with local repo: scion-apps/webapp/web")
		CheckFatal(err)
		return
	}
	checkPath(options.StaticRoot)

	// logging
	logDirPath := ensurePath(options.StaticRoot, "logs")
	checkPath(options.ScionLogs)
	log.Root().SetHandler(log.MultiHandler(
		log.LvlFilterHandler(log.LvlDebug,
			log.StreamHandler(os.Stderr, fmt15.Fmt15Format(fmt15.ColorMap))),
		log.LvlFilterHandler(log.LvlInfo,
			log.Must.FileHandler(path.Join(logDirPath, fmt.Sprintf("%s.log", id)),
				fmt15.Fmt15Format(nil)))))
	log.Info("======================> Webapp started")

	// prepare templates
	templates = prepareTemplates(options.StaticRoot)
	// open and manage database
	dbpath := path.Join(options.StaticRoot, "webapp.db")
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
	ensurePath(options.StaticRoot, "data")
	ensurePath(options.StaticRoot, "data/images")

	checkPath(options.ScionRoot)
	checkPath(options.ScionGen)
	checkPath(options.ScionGenCache)
	initLocalIaOptions()
	log.Info("IA loaded:", "myIa", settings.MyIA)

	// generate client/server default
	lib.GenClientNodeDefaults(&options, settings.MyIA)
	lib.GenServerNodeDefaults(&options, localIAs)

	checkPath(options.AppsRoot)
	checkPath(options.ScionBin)
	appsBuildCheck("bwtester")
	appsBuildCheck("camerapp")
	appsBuildCheck("sensorapp")
	appsBuildCheck("echo")
	appsBuildCheck("traceroute")

	initServeHandlers()
	log.Info(fmt.Sprintf("Browser access: at http://%s:%d.", browserAddr, *port))
	checkPath(options.BrowseRoot)
	log.Info("File browser root:", "root", options.BrowseRoot)
	log.Info(fmt.Sprintf("Listening on %s:%d...", *addr, *port))
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", *addr, *port), logRequestHandler(http.DefaultServeMux))
	CheckFatal(err)
}

// load list of locally available IAs and determine user choices
func initLocalIaOptions() {
	localIAs = lib.ScanLocalIAs(&options)
	settings = lib.ReadUserSetting(&options)

	// if read myia not in list, pick default
	iaExists := lib.StringInSlice(localIAs, settings.MyIA)

	// check for saved MyIA or use first available
	if !iaExists {
		if len(localIAs) > 0 {
			settings.MyIA = localIAs[0]
		} else {
			settings.MyIA = ""
		}
	}
	lib.WriteUserSetting(&options, settings)
}

func initServeHandlers() {
	serveExact("/favicon.ico", "./favicon.ico")
	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/about", aboutHandler)
	http.HandleFunc("/apps", appsHandler)
	http.HandleFunc("/astopo", astopoHandler)
	http.HandleFunc("/crt", crtHandler)
	http.HandleFunc("/trc", trcHandler)
	fsStatic := http.FileServer(http.Dir(path.Join(options.StaticRoot, "static")))
	http.Handle("/static/", http.StripPrefix("/static/", fsStatic))
	fsData := http.FileServer(http.Dir(path.Join(options.StaticRoot, "data")))
	http.Handle("/data/", http.StripPrefix("/data/", fsData))
	fsFileBrowser := http.FileServer(http.Dir(options.BrowseRoot))
	http.Handle("/files/", http.StripPrefix("/files/", fsFileBrowser))

	http.HandleFunc("/command", commandHandler)
	http.HandleFunc("/imglast", findImageHandler)
	http.HandleFunc("/txtlast", findImageInfoHandler)
	http.HandleFunc("/getnodes", getNodesHandler)
	http.HandleFunc("/getbwbytime", getBwByTimeHandler)
	http.HandleFunc("/healthcheck", healthCheckHandler)
	http.HandleFunc("/dirview", dirViewHandler)
	http.HandleFunc("/getechobytime", getEchoByTimeHandler)
	http.HandleFunc("/gettraceroutebytime", getTracerouteByTimeHandler)
	http.HandleFunc("/getias", getIAsHandler)
	http.HandleFunc("/setuseropt", setUserOptionsHandler)

	//ported from scion-viz
	http.HandleFunc("/config", lib.ConfigHandler)
	http.HandleFunc("/labels", lib.LabelsHandler)
	http.HandleFunc("/locations", lib.LocationsHandler)
	http.HandleFunc("/geolocate", lib.GeolocateHandler)
	http.HandleFunc("/getpathtopo", getPathInfoHandler)
	http.HandleFunc("/getastopo", lib.AsTopoHandler)
	http.HandleFunc("/getcrt", getCrtInfoHandler)
	http.HandleFunc("/gettrc", getTrcInfoHandler)
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
	display(w, "health", &Page{Title: "SCIONLab Health", MyIA: settings.MyIA})
}

func errorHandler(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)
	display(w, "error", &Page{Title: "SCIONLab Error", MyIA: settings.MyIA})
}

func dirViewHandler(w http.ResponseWriter, r *http.Request) {
	display(w, "dirview", &Page{Title: "SCIONLab Files", MyIA: settings.MyIA})
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	display(w, "about", &Page{Title: "SCIONLab About", MyIA: settings.MyIA})
}

func appsHandler(w http.ResponseWriter, r *http.Request) {
	display(w, "apps", &Page{Title: "SCIONLab Apps", MyIA: settings.MyIA})
}

func astopoHandler(w http.ResponseWriter, r *http.Request) {
	display(w, "astopo", &Page{Title: "SCIONLab AS Topology", MyIA: settings.MyIA})
}

func crtHandler(w http.ResponseWriter, r *http.Request) {
	display(w, "crt", &Page{Title: "SCIONLab Cert", MyIA: settings.MyIA})
}

func trcHandler(w http.ResponseWriter, r *http.Request) {
	display(w, "trc", &Page{Title: "SCIONLab TRC", MyIA: settings.MyIA})
}

// There're three CmdItem, BwTestItem, EchoItem and TracerouteItem
func parseRequest2CmdItem(r *http.Request, appSel string) (model.CmdItem, string) {
	addlOpt := r.PostFormValue("addl_opt")

	if appSel == "echo" {
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
	} else if appSel == "traceroute" { // ###need to be confirmed###
		d := model.TracerouteItem{}
		d.SIa = r.PostFormValue("ia_ser")
		d.CIa = r.PostFormValue("ia_cli")
		d.SAddr = r.PostFormValue("addr_ser")
		d.CAddr = r.PostFormValue("addr_cli")

		// TODO: parse timeout flag for traceroute
		d.Timeout = 2
		return d, addlOpt
	} else {
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
}

// d could be either model.BwTestItem, model.EchoItem or model.TracerouteItem
func parseCmdItem2Cmd(dOrinial model.CmdItem, appSel string, pathStr string) []string {
	var command []string
	var isdCli int
	installpath := getClientLocationBin(appSel)
	log.Info(fmt.Sprintf("App tag is %s...", appSel))
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
			log.Error("Parsing error, CmdItem category doesn't match its name")
			return nil
		}
		optApp := "echo"
		optLocal := fmt.Sprintf("-local=%s,[%s]", d.CIa, d.CAddr)
		optRemote := fmt.Sprintf("-remote=%s,[%s]", d.SIa, d.SAddr)
		optCount := fmt.Sprintf("-c=%d", d.Count)
		optTimeout := fmt.Sprintf("-timeout=%fs", d.Timeout)
		optInterval := fmt.Sprintf("-interval=%fs", d.Interval)
		command = append(command, installpath, optApp, optRemote, optLocal, optCount, optTimeout, optInterval)
		if len(pathStr) > 0 {
			// if path choice provided, use interactive mode
			command = append(command, "-i")
		}
		isdCli, _ = strconv.Atoi(strings.Split(d.CIa, "-")[0])

	case "traceroute":
		d, ok := dOrinial.(model.TracerouteItem)
		if !ok {
			log.Error("Parsing error, CmdItem category doesn't match its name")
			return nil
		}
		optApp := "tr"
		optLocal := fmt.Sprintf("-local=%s,[%s]", d.CIa, d.CAddr)
		optRemote := fmt.Sprintf("-remote=%s,[%s]", d.SIa, d.SAddr)
		optTimeout := fmt.Sprintf("-timeout=%fs", d.Timeout)
		command = append(command, installpath, optApp, optRemote, optLocal, optTimeout)
		if len(pathStr) > 0 {
			// if path choice provided, use interactive mode
			command = append(command, "-i")
		}
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
		// continuous run command, either bwtest, echo or traceroute
		contCmdTimeKeepAlive = time.Now()
		contCmdRequest = r
		contCmdInterval = interval
		var t contCmd
		if appSel == "bwtester" {
			t = bwTest
		} else if appSel == "echo" {
			t = echo
		} else if appSel == "traceroute" {
			t = traceroute
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

// Could either be bwtest, echo or traceroute
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

	stdin, err := cmd.StdinPipe()
	CheckError(err)
	stderr, err := cmd.StderrPipe()
	CheckError(err)
	stdout, err := cmd.StdoutPipe()
	CheckError(err)
	reader := io.MultiReader(stdout, stderr)

	err = cmd.Start()
	if CheckError(err) {
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
		CheckError(errors.New("App missing, build all apps with 'deps.sh' and 'make install'."))
	} else {
		log.Info(fmt.Sprintf("Existing install, found %s...", app))
	}
}

// Parses html selection and returns current working directory for execution.
func getClientCwd(app string) string {
	var cwd string
	switch app {
	case "sensorapp":
		cwd = path.Join(options.StaticRoot, "data")
	case "camerapp":
		cwd = path.Join(options.StaticRoot, "data/images")
	case "bwtester":
		cwd = path.Join(options.StaticRoot, "data")
	case "echo", "traceroute":
		cwd = path.Join(options.ScionBin, ".")
	}
	return cwd
}

// Parses html selection and returns name of app binary.
func getClientLocationBin(app string) string {
	var binname string
	switch app {
	case "sensorapp":
		binname = path.Join(options.AppsRoot, "sensorfetcher")
	case "camerapp":
		binname = path.Join(options.AppsRoot, "imagefetcher")
	case "bwtester":
		binname = path.Join(options.AppsRoot, "bwtestclient")
	case "echo", "traceroute":
		binname = path.Join(options.ScionBin, "scmp")
	}
	return binname
}

// Handles piping command line output to logs, database, and http response writer.
func writeCmdOutput(w http.ResponseWriter, reader io.Reader, stdin io.WriteCloser, d model.CmdItem, appSel string, pathStr string, cmd *exec.Cmd) {
	// regex to find matching path in interactive mode
	var errMsg string
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

	var re = regexp.MustCompile(reRemoveAnsi)
	pathsAvail := false
	jsonBuf := []byte(``)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		// read each line from stdout
		line := re.ReplaceAllString(scanner.Text(), "")
		// log.Info(line)

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
		lib.WriteCmdCsv(d, &options, appSel)
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
		lib.WriteCmdCsv(d, &options, appSel)
	}

	if appSel == "traceroute" {
		// parse traceroute data/error
		d, ok := d.(model.TracerouteItem)
		if !ok {
			log.Error("Parsing error, CmdItem category doesn't match its name")
			return
		}
		lib.ExtractTracerouteRespData(string(jsonBuf), &d, start)
		if len(errMsg) > 0 {
			d.Error = errMsg
		}
		// store in database
		err := model.StoreTracerouteItem(&d)
		if CheckError(err) {
			d.Error = err.Error()
		}
		lib.WriteCmdCsv(d, &options, appSel)
	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	lib.HealthCheckHandler(w, r, &options, settings.MyIA)
}

func getBwByTimeHandler(w http.ResponseWriter, r *http.Request) {
	lib.GetBwByTimeHandler(w, r, contCmdActive)
}

func getEchoByTimeHandler(w http.ResponseWriter, r *http.Request) {
	lib.GetEchoByTimeHandler(w, r, contCmdActive)
}

func getTracerouteByTimeHandler(w http.ResponseWriter, r *http.Request) {
	lib.GetTracerouteByTimeHandler(w, r, contCmdActive)
}

// Handles locating most recent image formatting it for graphic display in response.
func findImageHandler(w http.ResponseWriter, r *http.Request) {
	lib.FindImageHandler(w, r, &options, browserAddr, *port)
}

func findImageInfoHandler(w http.ResponseWriter, r *http.Request) {
	lib.FindImageInfoHandler(w, r, &options)
}

func getTrcInfoHandler(w http.ResponseWriter, r *http.Request) {
	lib.TrcHandler(w, r, &options)
}

func getCrtInfoHandler(w http.ResponseWriter, r *http.Request) {
	lib.CrtHandler(w, r, &options)
}

func getPathInfoHandler(w http.ResponseWriter, r *http.Request) {
	lib.PathTopoHandler(w, r, &options)
}

func getNodesHandler(w http.ResponseWriter, r *http.Request) {
	lib.GetNodesHandler(w, r, &options)
}

func getIAsHandler(w http.ResponseWriter, r *http.Request) {
	// in:nil, out:list[ias]
	iasJSON, _ := json.Marshal(localIAs)
	fmt.Fprintf(w, string(iasJSON))
}

func setUserOptionsHandler(w http.ResponseWriter, r *http.Request) {
	// in:myIA , out:nil, set locally
	myIa := r.PostFormValue("myIA")
	settings = lib.ReadUserSetting(&options)
	settings.MyIA = myIa

	// save myIA to file
	lib.WriteUserSetting(&options, settings)
	lib.GenClientNodeDefaults(&options, settings.MyIA)
	log.Info("IA set:", "myIa", settings.MyIA)
}
