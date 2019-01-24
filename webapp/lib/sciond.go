package lib

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "path"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"

    "github.com/scionproto/scion/go/lib/addr"
    "github.com/scionproto/scion/go/lib/common"
    "github.com/scionproto/scion/go/lib/sciond"
    "github.com/scionproto/scion/go/lib/snet"
    "github.com/scionproto/scion/go/lib/spath/spathmeta"
    "github.com/scionproto/scion/go/proto"
)

// Configuations to save. Zeroing out any of these placeholders will cause the
// webserver to request a fresh external copy to keep locally.
var cConfig string
var cLabels string
var cNodes string
var cGeoLoc string

func returnError(w http.ResponseWriter, err error) {
    log.Println("error: " + err.Error())
    fmt.Fprintf(w, `{"err":`+strconv.Quote(err.Error())+`}`)
}

// sciond data sources and calls

// PathTopoHandler handles requests for paths, returning results from sciond.
func PathTopoHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    SIa := r.PostFormValue("ia_ser")
    CIa := r.PostFormValue("ia_cli")
    SAddr := r.PostFormValue("addr_ser")
    CAddr := r.PostFormValue("addr_cli")
    SPort, _ := strconv.Atoi(r.PostFormValue("port_ser"))
    CPort, _ := strconv.Atoi(r.PostFormValue("port_cli"))

    optClient := fmt.Sprintf("%s,[%s]:%d", CIa, CAddr, CPort)
    optServer := fmt.Sprintf("%s,[%s]:%d", SIa, SAddr, SPort)
    clientCCAddr, _ := snet.AddrFromString(optClient)
    serverCCAddr, _ := snet.AddrFromString(optServer)

    if snet.DefNetwork == nil {
        dispatcherPath := "/run/shm/dispatcher/default.sock"

        var sciondPath string
        isdCli, _ := strconv.Atoi(strings.Split(CIa, "-")[0])
        if isdCli < 16 {
            sciondPath = sciond.GetDefaultSCIONDPath(&clientCCAddr.IA)
        } else {
            sciondPath = sciond.GetDefaultSCIONDPath(nil)
        }

        err := snet.Init(clientCCAddr.IA, sciondPath, dispatcherPath)
        if err != nil {
            returnError(w, err)
            return
        }
    }

    paths := getPaths(*clientCCAddr, *serverCCAddr)
    if len(paths) == 0 {
        returnError(w, fmt.Errorf("No paths from %s to %s", clientCCAddr.IA,
            serverCCAddr.IA))
        return
    }

    jsonPathInfo, _ := json.Marshal(paths)
    log.Printf("paths: %s\n", jsonPathInfo)

    fmt.Fprintf(w, fmt.Sprintf(`{"paths":%s}`, jsonPathInfo))
}

func getPaths(local snet.Addr, remote snet.Addr) []*spathmeta.AppPath {
    pathMgr := snet.DefNetwork.PathResolver()
    pathSet := pathMgr.Query(context.Background(), local.IA, remote.IA)
    var appPaths []*spathmeta.AppPath
    for _, path := range pathSet {
        appPaths = append(appPaths, path)
    }
    return appPaths
}

// AsTopoHandler handles requests for AS data, returning results from sciond.
func AsTopoHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    CIa := r.PostFormValue("src")
    ia, err := addr.IAFromString(CIa)
    if err != nil {
        returnError(w, err)
        return
    }
    if snet.DefNetwork == nil {
        dispatcherPath := "/run/shm/dispatcher/default.sock"

        var sciondPath string
        isdCli, _ := strconv.Atoi(strings.Split(CIa, "-")[0])
        if isdCli < 16 {
            sciondPath = sciond.GetDefaultSCIONDPath(&ia)
        } else {
            sciondPath = sciond.GetDefaultSCIONDPath(nil)
        }

        err := snet.Init(ia, sciondPath, dispatcherPath)
        if err != nil {
            returnError(w, err)
            return
        }
    }

    c := snet.DefNetwork.Sciond()

    asir, err := c.ASInfo(context.Background(), addr.IA{})
    if err != nil {
        returnError(w, err)
        return
    }
    ajsonInfo, _ := json.Marshal(asir)
    log.Printf("asinfos: %s\n", ajsonInfo)

    ifirs, err := c.IFInfo(context.Background(), []common.IFIDType{})
    if err != nil {
        returnError(w, err)
        return
    }
    ijsonInfo, _ := json.Marshal(ifirs)
    log.Printf("ifinfos: %s\n", ijsonInfo)

    svcirs, err := c.SVCInfo(context.Background(), []proto.ServiceType{
        proto.ServiceType_bs, proto.ServiceType_ps, proto.ServiceType_cs,
        proto.ServiceType_sb, proto.ServiceType_sig, proto.ServiceType_ds})
    if err != nil {
        returnError(w, err)
        return
    }
    sjsonInfo, _ := json.Marshal(svcirs)
    log.Printf("svcinfos: %s\n", sjsonInfo)

    fmt.Fprintf(w, fmt.Sprintf(`{"as_info":%s,"if_info":%s,"svc_info":%s}`,
        ajsonInfo, ijsonInfo, sjsonInfo))
}

// TrcHandler handles requests for all local trust root data.
func TrcHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    CIa := r.PostFormValue("src")
    raw, err := loadJSONCerts(CIa, "*.trc")
    if err != nil {
        returnError(w, err)
        return
    }
    jsonInfo, _ := json.Marshal(raw)
    log.Printf("jsonInfo: %s\n", jsonInfo)

    fmt.Fprintf(w, string(raw))
}

// CrtHandler handles requests for all local certificate data.
func CrtHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    CIa := r.PostFormValue("src")
    raw, err := loadJSONCerts(CIa, "*.crt")
    if err != nil {
        returnError(w, err)
        return
    }
    jsonInfo, _ := json.Marshal(raw)
    log.Printf("jsonInfo: %s\n", jsonInfo)

    fmt.Fprintf(w, string(raw))
}

func loadJSONCerts(src, pattern string) ([]byte, error) {
    ia, err := addr.IAFromString(src)
    certDir := path.Join(GOPATH, SCIONROOT,
        fmt.Sprintf("gen/ISD%d/AS%s/endhost/certs", ia.I, ia.A.FileFmt()))
    cacheDir := path.Join(GOPATH, SCIONROOT, "gen-cache")
    files, err := filepath.Glob(fmt.Sprintf("%s/%s", certDir, pattern))
    if err != nil {
        return nil, err
    }
    cachedFiles, err := filepath.Glob(fmt.Sprintf("%s/%s", cacheDir, pattern))
    if err != nil {
        return nil, err
    }
    filesJSON, _ := loadJSONFiles(files)
    cachedJSON, _ := loadJSONFiles(cachedFiles)

    jsonBuf := []byte(`{ `)
    jsonBuf = append(jsonBuf, filesJSON...)
    if len(filesJSON) > 0 && len(cachedJSON) > 0 {
        jsonBuf = append(jsonBuf, []byte(`, `)...)
    }
    if len(cachedJSON) > 0 {
        jsonBuf = append(jsonBuf, []byte(`"Cache": {`)...)
        jsonBuf = append(jsonBuf, cachedJSON...)
        jsonBuf = append(jsonBuf, []byte(` }`)...)
    }
    jsonBuf = append(jsonBuf, []byte(`}`)...)

    return jsonBuf, nil
}

func loadJSONFiles(files []string) ([]byte, error) {
    idx := 0
    var jsonBuf []byte
    for _, file := range files {
        raw, err := ioutil.ReadFile(file)
        if err != nil {
            return nil, err
        }
        // concat raw files...
        if idx > 0 {
            jsonBuf = append(jsonBuf, []byte(`, `)...)
        }
        jsonBuf = append(jsonBuf, []byte(`"`+filepath.Base(file)+`": `)...)
        jsonBuf = append(jsonBuf, raw...)
        idx++
    }
    return jsonBuf, nil
}

// remote data files and services

// ConfigHandler handles requests for configurable, centralized data sources.
func ConfigHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    debug, _ := strconv.ParseBool(strings.Join(r.Form["debug"], ""))
    projectID := "my-project-1470640410708"
    url := fmt.Sprintf("https://%s.appspot.com/getconfig", projectID)
    if len(cConfig) == 0 {
        if debug {
            raw := loadTestFile("tests/asviz/config-d.json")
            cConfig = string(raw)
        } else {
            buf := new(bytes.Buffer)
            resp, err := http.Post(url, "application/json", buf)
            if err != nil {
                returnError(w, err)
                return
            }
            defer resp.Body.Close()
            body, _ := ioutil.ReadAll(resp.Body)
            cConfig = string(body)
        }
    }
    fmt.Println(cConfig)
    fmt.Fprintf(w, cConfig)
}

// LabelsHandler handles AS label requests, using exernal request when needed.
func LabelsHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    debug, _ := strconv.ParseBool(strings.Join(r.Form["debug"], ""))
    url := strings.Join(r.Form["labels_json_url"], "")
    if len(cLabels) == 0 {
        if debug {
            raw := loadTestFile("tests/asviz/labels-d.json")
            cLabels = string(raw)
        } else {
            resp, err := http.Get(url)
            if err != nil {
                returnError(w, err)
                return
            }
            defer resp.Body.Close()
            body, _ := ioutil.ReadAll(resp.Body)
            cLabels = string(body)
        }
    }
    fmt.Println(cLabels)
    fmt.Fprintf(w, cLabels)
}

// LocationsHandler handles AS location requests, using exernal request when needed.
func LocationsHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    debug, _ := strconv.ParseBool(strings.Join(r.Form["debug"], ""))
    url := strings.Join(r.Form["nodes_xml_url"], "")
    if len(cNodes) == 0 {
        if debug {
            raw := loadTestFile("tests/asviz/nodes-d.xml")
            cNodes = string(raw)
        } else {
            resp, err := http.Get(url)
            if err != nil {
                returnError(w, err)
                return
            }
            defer resp.Body.Close()
            body, _ := ioutil.ReadAll(resp.Body)
            cNodes = string(body)
        }
    }
    fmt.Println(cNodes)
    fmt.Fprintf(w, cNodes)
}

// GeolocateHandler handles geolocation requests, using exernal request when needed.
func GeolocateHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    debug, _ := strconv.ParseBool(strings.Join(r.Form["debug"], ""))
    geoAPIKey := strings.Join(r.Form["google_geolocation_apikey"], "")
    url := fmt.Sprintf(
        "https://www.googleapis.com/geolocation/v1/geolocate?key=%s", geoAPIKey)
    if len(cGeoLoc) == 0 {
        if debug {
            raw := loadTestFile("tests/asviz/geolocate-d.json")
            cGeoLoc = string(raw)
        } else {
            buf := new(bytes.Buffer)
            resp, err := http.Post(url, "application/json", buf)
            if err != nil {
                returnError(w, err)
                return
            }
            defer resp.Body.Close()
            body, _ := ioutil.ReadAll(resp.Body)
            cGeoLoc = string(body)
        }
    }
    fmt.Println(cGeoLoc)
    fmt.Fprintf(w, cGeoLoc)
}

func loadTestFile(testpath string) []byte {
    _, srcfile, _, _ := runtime.Caller(0)
    srcpath := path.Dir(srcfile)

    var fp = path.Join(srcpath, "..", testpath)
    raw, err := ioutil.ReadFile(fp)
    if err != nil {
        log.Println("ioutil.ReadFile() error: " + err.Error())
    }
    return raw
}
