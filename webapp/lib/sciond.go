package lib

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "net/http"
    "os"
    "path"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"

    log "github.com/inconshreveable/log15"
    pathdb "github.com/netsec-ethz/scion-apps/webapp/models/path"
    . "github.com/netsec-ethz/scion-apps/webapp/util"
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
        if CheckError(err) {
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
    log.Debug("PathTopoHandler:", "jsonPathInfo", string(jsonPathInfo))

    // load segments from paths database
    var dbSrcFile = findDBFilename(clientCCAddr.IA)
    dbTmpFile := copyDBToTemp(dbSrcFile)
    // since http.ListenAndServe() blocks, ensure we generate a local db object
    // which will live only during the http call
    db := pathdb.InitDB(dbTmpFile)
    defer func() {
        pathdb.CloseDB(db)
        removeAllDir(filepath.Dir(dbTmpFile))
    }()
    segTypes := pathdb.ReadSegTypesAll(db)
    segments := pathdb.ReadSegmentsAll(db, segTypes)

    jsonSegsInfo, _ := json.Marshal(segments)
    log.Debug("PathTopoHandler:", "jsonSegsInfo", string(jsonSegsInfo))

    fmt.Fprintf(w, fmt.Sprintf(`{"paths":%s,"segments":%s}`,
        jsonPathInfo, jsonSegsInfo))
}

func findDBFilename(ia addr.IA) string {
    filenames, err := filepath.Glob(filepath.Join(
        GOPATH, SCIONROOT, "gen-cache", "ps*path.db"))
    CheckError(err)
    if len(filenames) == 1 {
        return filenames[0]
    }
    pathDBFileName := fmt.Sprintf("ps%s-1.path.db", ia.FileFmt(false))
    return filepath.Join(GOPATH, SCIONROOT, "gen-cache", pathDBFileName)
}

// returns the name of the created file
func copyDBToTemp(filename string) string {
    copyOneFile := func(dstDir, srcFileName string) error {
        src, err := os.Open(srcFileName)
        if CheckError(err) {
            return fmt.Errorf("Cannot open %s: %v", srcFileName, err)
        }
        defer src.Close()
        dstFilename := filepath.Join(dstDir, filepath.Base(srcFileName))
        dst, err := os.Create(dstFilename)
        if CheckError(err) {
            return fmt.Errorf("Cannot open %s: %v", dstFilename, err)
        }
        defer dst.Close()
        _, err = io.Copy(dst, src)
        if CheckError(err) {
            return fmt.Errorf("Cannot copy %s to %s: %v", srcFileName, dstFilename, err.Error())
        }
        return nil
    }
    dirName, err := ioutil.TempDir("/tmp", "sciond_dump")
    if CheckError(err) {
        return err.Error()
    }

    err = copyOneFile(dirName, filename)
    if CheckError(err) {
        fmt.Fprintf(os.Stderr, "No panic: %v", err)
    }
    err = copyOneFile(dirName, filename+"-wal")
    CheckError(err)
    return filepath.Join(dirName, filepath.Base(filename))
}

func removeAllDir(dirName string) {
    err := os.RemoveAll(dirName)
    CheckError(err)
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
    if CheckError(err) {
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
        if CheckError(err) {
            returnError(w, err)
            return
        }
    }

    c := snet.DefNetwork.Sciond()

    asir, err := c.ASInfo(context.Background(), addr.IA{})
    if CheckError(err) {
        returnError(w, err)
        return
    }
    ajsonInfo, _ := json.Marshal(asir)
    log.Debug("AsTopoHandler:", "ajsonInfo", string(ajsonInfo))

    ifirs, err := c.IFInfo(context.Background(), []common.IFIDType{})
    if CheckError(err) {
        returnError(w, err)
        return
    }
    ijsonInfo, _ := json.Marshal(ifirs)
    log.Debug("AsTopoHandler:", "ijsonInfo", string(ijsonInfo))

    svcirs, err := c.SVCInfo(context.Background(), []proto.ServiceType{
        proto.ServiceType_bs, proto.ServiceType_ps, proto.ServiceType_cs,
        proto.ServiceType_sb, proto.ServiceType_sig, proto.ServiceType_ds})
    if CheckError(err) {
        returnError(w, err)
        return
    }
    sjsonInfo, _ := json.Marshal(svcirs)
    log.Debug("AsTopoHandler:", "sjsonInfo", string(sjsonInfo))

    fmt.Fprintf(w, fmt.Sprintf(`{"as_info":%s,"if_info":%s,"svc_info":%s}`,
        ajsonInfo, ijsonInfo, sjsonInfo))
}

// TrcHandler handles requests for all local trust root data.
func TrcHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    CIa := r.PostFormValue("src")
    raw, err := loadJSONCerts(CIa, "*.trc")
    if CheckError(err) {
        returnError(w, err)
        return
    }
    log.Debug("TrcHandler:", "trcInfo", string(raw))
    fmt.Fprintf(w, string(raw))
}

// CrtHandler handles requests for all local certificate data.
func CrtHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    CIa := r.PostFormValue("src")
    raw, err := loadJSONCerts(CIa, "*.crt")
    if CheckError(err) {
        returnError(w, err)
        return
    }
    log.Debug("CrtHandler:", "crtInfo", string(raw))
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
            if CheckError(err) {
                returnError(w, err)
                return
            }
            defer resp.Body.Close()
            body, _ := ioutil.ReadAll(resp.Body)
            cConfig = string(body)
        }
        log.Debug("ConfigHandler:", "cached", cConfig)
    }
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
            if CheckError(err) {
                returnError(w, err)
                return
            }
            defer resp.Body.Close()
            body, _ := ioutil.ReadAll(resp.Body)
            cLabels = string(body)
        }
        log.Debug("LabelsHandler:", "cached", cLabels)
    }
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
            if CheckError(err) {
                returnError(w, err)
                return
            }
            defer resp.Body.Close()
            body, _ := ioutil.ReadAll(resp.Body)
            cNodes = string(body)
        }
        log.Debug("LocationsHandler:", "cached", cNodes)
    }
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
            if CheckError(err) {
                returnError(w, err)
                return
            }
            defer resp.Body.Close()
            body, _ := ioutil.ReadAll(resp.Body)
            cGeoLoc = string(body)
        }
        log.Debug("GeolocateHandler:", "cached", cGeoLoc)
    }
    fmt.Fprintf(w, cGeoLoc)
}

func loadTestFile(testpath string) []byte {
    _, srcfile, _, _ := runtime.Caller(0)
    srcpath := path.Dir(srcfile)

    var fp = path.Join(srcpath, "..", testpath)
    raw, err := ioutil.ReadFile(fp)
    CheckError(err)
    return raw
}
