// Copyright 2019 ETH Zurich
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

package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	log "github.com/inconshreveable/log15"
	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	pathdb "github.com/netsec-ethz/scion-apps/webapp/models/path"
	. "github.com/netsec-ethz/scion-apps/webapp/util"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
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

func returnPathHandler(w http.ResponseWriter, pathJSON []byte, segJSON []byte, err error) {
	var buffer bytes.Buffer
	buffer.WriteString(`{"src":"sciond"`)
	if pathJSON != nil {
		buffer.WriteString(fmt.Sprintf(`,"paths":%s`, pathJSON))
	}
	if segJSON != nil {
		buffer.WriteString(fmt.Sprintf(`,"segments":%s`, segJSON))
	}
	if err != nil {
		buffer.WriteString(fmt.Sprintf(`,"err":%s`, strconv.Quote(err.Error())))
	}
	buffer.WriteString(`}`)
	fmt.Fprintf(w, buffer.String())
}

func getNetworkByIA(iaCli string) (*snet.SCIONNetwork, error) {
	ia, err := addr.IAFromString(iaCli)
	if CheckError(err) {
		return nil, err
	}
	dispatcherPath := "/run/shm/dispatcher/default.sock"
	sciondPath := scionutil.GetSCIONDPath(&ia)
	if snet.DefNetwork == nil {
		err := snet.Init(ia, sciondPath, reliable.NewDispatcherService(dispatcherPath))
		if CheckError(err) {
			return nil, err
		}
	}
	network, err := snet.NewNetwork(ia, sciondPath,
		reliable.NewDispatcherService(dispatcherPath))

	if CheckError(err) {
		return nil, err
	}
	return network, nil
}

// sciond data sources and calls

// PathTopoHandler handles requests for paths, returning results from sciond.
func PathTopoHandler(w http.ResponseWriter, r *http.Request, options *CmdOptions) {
	r.ParseForm()
	SIa := r.PostFormValue("ia_ser")
	CIa := r.PostFormValue("ia_cli")
	SAddr := r.PostFormValue("addr_ser")
	CAddr := r.PostFormValue("addr_cli")
	SPort, _ := strconv.Atoi(r.PostFormValue("port_ser"))
	CPort, _ := strconv.Atoi(r.PostFormValue("port_cli"))

	// src and dst must be different
	if SIa == CIa {
		returnError(w, errors.New("Source IA and destination IA are the same."))
		return
	}
	optClient := fmt.Sprintf("%s,[%s]:%d", CIa, CAddr, CPort)
	optServer := fmt.Sprintf("%s,[%s]:%d", SIa, SAddr, SPort)
	clientCCAddr, _ := snet.AddrFromString(optClient)
	serverCCAddr, _ := snet.AddrFromString(optServer)

	network, err := getNetworkByIA(CIa)
	if CheckError(err) {
		returnError(w, err)
		return
	}

	paths, err := getPathsJSON(*clientCCAddr, *serverCCAddr, *network)
	if CheckError(err) {
		returnError(w, err)
		return
	}
	log.Debug("PathTopoHandler:", "paths", string(paths))

	// Since segments data is supplimentary to paths data, if segments data
	// fails, provide the error, but we must still allow paths data to return.
	segments, err := getSegmentsJSON(*clientCCAddr, options)
	if CheckError(err) {
		returnPathHandler(w, paths, nil, err)
		return
	}
	log.Debug("PathTopoHandler:", "segments", string(segments))

	returnPathHandler(w, paths, segments, err)
}

func getSegmentsJSON(local snet.Addr, options *CmdOptions) ([]byte, error) {
	// load segments from paths database
	var dbSrcFile = findDBFilename(local.IA, options)
	dbTmpFile, err := copyDBToTemp(dbSrcFile)
	if err != nil {
		return nil, err
	}
	// since http.ListenAndServe() blocks, ensure we generate a local db object
	// which will live only during the http call
	db, err := pathdb.InitDB(dbTmpFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		pathdb.CloseDB(db)
		removeAllDir(filepath.Dir(dbTmpFile))
	}()
	segTypes, err := pathdb.ReadSegTypesAll(db)
	if err != nil {
		return nil, err
	}
	segments, err := pathdb.ReadSegmentsAll(db, segTypes)
	if err != nil {
		return nil, err
	}
	sort.Slice(segments, func(i, j int) bool {
		// sort by segment type, then shortest # hops
		if segments[i].SegType < segments[j].SegType {
			return true
		}
		if segments[i].SegType > segments[j].SegType {
			return false
		}
		return len(segments[i].Interfaces) < len(segments[j].Interfaces)
	})
	jsonSegsInfo, err := json.Marshal(segments)
	if err != nil {
		return nil, err
	}
	return jsonSegsInfo, nil
}

func findDBFilename(ia addr.IA, options *CmdOptions) string {
	filenames, err := filepath.Glob(filepath.Join(options.ScionGenCache, "ps*path.db"))
	CheckError(err)
	if len(filenames) == 1 {
		return filenames[0]
	}
	pathDBFileName := fmt.Sprintf("ps%s-1.path.db", ia.FileFmt(false))
	return filepath.Join(options.ScionGenCache, pathDBFileName)
}

// returns the name of the created file
func copyDBToTemp(filename string) (string, error) {
	copyOneFile := func(dstDir, srcFileName string) error {
		src, err := os.Open(srcFileName)
		if err != nil {
			return err
		}
		defer src.Close()
		dstFilename := filepath.Join(dstDir, filepath.Base(srcFileName))
		dst, err := os.Create(dstFilename)
		if err != nil {
			return err
		}
		defer dst.Close()
		_, err = io.Copy(dst, src)
		if err != nil {
			return err
		}
		return nil
	}
	dirName, err := ioutil.TempDir("/tmp", "sciond_dump")
	if err != nil {
		return "", err
	}
	err = copyOneFile(dirName, filename)
	if err != nil {
		return "", err
	}
	err = copyOneFile(dirName, filename+"-wal")
	if err != nil {
		return "", err
	}
	return filepath.Join(dirName, filepath.Base(filename)), nil
}

func removeAllDir(dirName string) {
	err := os.RemoveAll(dirName)
	CheckError(err)
}

func getPathsJSON(local snet.Addr, remote snet.Addr, network snet.SCIONNetwork) ([]byte, error) {
	pathMgr := network.PathResolver()
	pathSet := pathMgr.Query(context.Background(), local.IA, remote.IA, sciond.PathReqFlags{})
	if len(pathSet) == 0 {
		return nil, fmt.Errorf("No paths from %s to %s", local.IA, remote.IA)
	}
	var appPaths []*spathmeta.AppPath
	for _, path := range pathSet {
		appPaths = append(appPaths, path)
	}
	sort.Slice(appPaths, func(i, j int) bool {
		// sort by shortest # hops, then by IA/interface
		if len(appPaths[i].Entry.Path.Interfaces) < len(appPaths[j].Entry.Path.Interfaces) {
			return true
		}
		if len(appPaths[i].Entry.Path.Interfaces) > len(appPaths[j].Entry.Path.Interfaces) {
			return false
		}
		return appPaths[i].Entry.String() < appPaths[j].Entry.String()
	})
	jsonPathInfo, err := json.Marshal(appPaths)
	if err != nil {
		return nil, err
	}
	return jsonPathInfo, nil
}

// AsTopoHandler handles requests for AS data, returning results from sciond.
func AsTopoHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	CIa := r.PostFormValue("src")

	network, err := getNetworkByIA(CIa)
	if CheckError(err) {
		returnError(w, err)
		return
	}
	c := network.Sciond()

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
func TrcHandler(w http.ResponseWriter, r *http.Request, options *CmdOptions) {
	r.ParseForm()
	CIa := r.PostFormValue("src")
	raw, err := loadJSONCerts(CIa, "*.trc", options)
	if CheckError(err) {
		returnError(w, err)
		return
	}
	log.Debug("TrcHandler:", "trcInfo", string(raw))
	fmt.Fprintf(w, string(raw))
}

// CrtHandler handles requests for all local certificate data.
func CrtHandler(w http.ResponseWriter, r *http.Request, options *CmdOptions) {
	r.ParseForm()
	CIa := r.PostFormValue("src")
	raw, err := loadJSONCerts(CIa, "*.crt", options)
	if CheckError(err) {
		returnError(w, err)
		return
	}
	log.Debug("CrtHandler:", "crtInfo", string(raw))
	fmt.Fprintf(w, string(raw))
}

func loadJSONCerts(src, pattern string, options *CmdOptions) ([]byte, error) {
	ia, err := addr.IAFromString(src)
	if err != nil {
		return nil, err
	}
	certDir := path.Join(options.ScionGen, fmt.Sprintf("ISD%d/AS%s/endhost/certs", ia.I, ia.A.FileFmt()))
	cacheDir := options.ScionGenCache
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
