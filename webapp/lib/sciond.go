package lib

import (
    "bytes"
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
)

func returnError(w http.ResponseWriter, err error) {
    log.Println("error: " + err.Error())
    fmt.Fprintf(w, `{"err":`+strconv.Quote(err.Error())+`}`)
}

// sciond data sources and calls

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
        returnError(w, fmt.Errorf("No paths from %s to %s!", clientCCAddr.IA,
            serverCCAddr.IA))
        return
    }

    jsonPathInfo, _ := json.Marshal(paths)
    log.Printf("paths: %s\n", jsonPathInfo)

    fmt.Fprintf(w, fmt.Sprintf(`{"paths":%s}`, jsonPathInfo))
}

func getPaths(local snet.Addr, remote snet.Addr) []*spathmeta.AppPath {
    pathMgr := snet.DefNetwork.PathResolver()
    pathSet := pathMgr.Query(local.IA, remote.IA)
    var appPaths []*spathmeta.AppPath
    for _, path := range pathSet {
        appPaths = append(appPaths, path)
    }
    return appPaths
}

func AsTopoHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    CIa := r.PostFormValue("src")
    CAddr := "120.0.0.1" // TODO debug r.PostFormValue("addr_cli")
    CPort, _ := 2, ""    // TODO debug strconv.Atoi(r.PostFormValue("port_cli"))
    optClient := fmt.Sprintf("%s,[%s]:%d", CIa, CAddr, CPort)

    log.Println("optClient: " + optClient)

    clientCCAddr, _ := snet.AddrFromString(optClient)

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

    sd := snet.DefNetwork.Sciond()
    c, err := sd.Connect()
    if err != nil {
        returnError(w, err)
        return
    }

    asir, err := c.ASInfo(addr.IA{})
    if err != nil {
        returnError(w, err)
        return
    }
    ajsonInfo, _ := json.Marshal(asir)
    log.Printf("asinfos: %s\n", ajsonInfo)

    ifirs, err := c.IFInfo([]common.IFIDType{})
    if err != nil {
        returnError(w, err)
        return
    }
    ijsonInfo, _ := json.Marshal(ifirs)
    log.Printf("ifinfos: %s\n", ijsonInfo)

    // kills sciond: sciond.SvcSB, sciond.SvcBR
    svcirs, err := c.SVCInfo([]sciond.ServiceType{
        sciond.SvcBS, sciond.SvcPS, sciond.SvcCS})
    if err != nil {
        returnError(w, err)
        return
    }
    sjsonInfo, _ := json.Marshal(svcirs)
    log.Printf("svcinfos: %s\n", sjsonInfo)

    fmt.Fprintf(w, fmt.Sprintf(`{"as_info":%s,"if_info":%s,"svc_info":%s}`,
        ajsonInfo, ijsonInfo, sjsonInfo))
}

func TrcHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    CIa := r.PostFormValue("src")
    raw, err := loadJsonCerts(CIa, "*.trc")
    if err != nil {
        returnError(w, err)
        return
    }
    jsonInfo, _ := json.Marshal(raw)
    log.Printf("jsonInfo: %s\n", jsonInfo)

    fmt.Fprintf(w, string(raw))
}

func CrtHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    CIa := r.PostFormValue("src")
    raw, err := loadJsonCerts(CIa, "*.crt")
    if err != nil {
        returnError(w, err)
        return
    }
    jsonInfo, _ := json.Marshal(raw)
    log.Printf("jsonInfo: %s\n", jsonInfo)

    fmt.Fprintf(w, string(raw))
}

func loadJsonCerts(src, pattern string) ([]byte, error) {
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

    idx := 0
    jsonBuf := []byte(`{ `)
    for _, file := range append(files, cachedFiles...) {
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
    jsonBuf = append(jsonBuf, []byte(` }`)...)
    return jsonBuf, nil
}

// remote data files and services

func ConfigHandler(w http.ResponseWriter, r *http.Request) {
    projectID := "my-project-1470640410708"
    url := fmt.Sprintf("https://%s.appspot.com/getconfig", projectID)
    buf := new(bytes.Buffer)
    resp, err := http.Post(url, "application/json", buf)
    if err != nil {
        returnError(w, err)
        return
    }
    defer resp.Body.Close()
    body, _ := ioutil.ReadAll(resp.Body)
    jsonResp := string(body)
    fmt.Println(jsonResp)
    fmt.Fprintf(w, jsonResp)
}

func LabelsHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    debug, _ := strconv.ParseBool(strings.Join(r.Form["debug"], ""))
    url := strings.Join(r.Form["labels_json_url"], "")
    var jsonResp string
    if debug {
        raw := loadTestFile("tests/asviz/labels-d.json")
        jsonResp = string(raw)
    } else {
        resp, err := http.Get(url)
        if err != nil {
            returnError(w, err)
            return
        }
        defer resp.Body.Close()
        body, _ := ioutil.ReadAll(resp.Body)
        jsonResp = string(body)
    }
    fmt.Println(jsonResp)
    fmt.Fprintf(w, jsonResp)
}

func LocationsHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    debug, _ := strconv.ParseBool(strings.Join(r.Form["debug"], ""))
    url := strings.Join(r.Form["nodes_xml_url"], "")
    var jsonResp string
    if debug {
        raw := loadTestFile("tests/asviz/nodes-d.xml")
        jsonResp = string(raw)
    } else {
        resp, err := http.Get(url)
        if err != nil {
            returnError(w, err)
            return
        }
        defer resp.Body.Close()
        body, _ := ioutil.ReadAll(resp.Body)
        jsonResp = string(body)
    }
    fmt.Println(jsonResp)
    fmt.Fprintf(w, jsonResp)
}

func GeolocateHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    debug, _ := strconv.ParseBool(strings.Join(r.Form["debug"], ""))
    geoAPIKey := strings.Join(r.Form["google_geolocation_apikey"], "")
    url := fmt.Sprintf(
        "https://www.googleapis.com/geolocation/v1/geolocate?key=%s", geoAPIKey)
    var jsonResp string
    if debug {
        raw := loadTestFile("tests/asviz/geolocate-d.json")
        jsonResp = string(raw)
    } else {
        buf := new(bytes.Buffer)
        resp, err := http.Post(url, "application/json", buf)
        if err != nil {
            returnError(w, err)
            return
        }
        defer resp.Body.Close()
        body, _ := ioutil.ReadAll(resp.Body)
        jsonResp = string(body)
    }
    fmt.Println(jsonResp)
    fmt.Fprintf(w, jsonResp)
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
