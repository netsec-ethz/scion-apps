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
// limitations under the License.

package lib

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	log "github.com/inconshreveable/log15"
	. "github.com/netsec-ethz/scion-apps/webapp/util"
	"github.com/pelletier/go-toml"
	"github.com/scionproto/scion/go/lib/sciond"
)

// default params for localhost testing
var listenAddrDef = "127.0.0.1"
var listenPortDef = 8000
var cliPortDef = 30001
var serPortDef = 30100
var serDefAddr = "127.0.0.2"

var cfgFileSerUser = "config/servers_user.json"
var cfgFileCliDef = "config/clients_default.json"
var cfgFileSerDef = "config/servers_default.json"

var topologyFile = "topology.json"
var sdToml = "sd.toml"
var sciondToml = "sciond.toml"

// command argument constants
var CMD_ADR = "a"
var CMD_PRT = "p"
var CMD_WEB = "srvroot"
var CMD_BRT = "r"
var CMD_SCG = "sgen"
var CMD_SCC = "sgenc"

// appsRoot is the root location of scionlab apps.
var GOPATH = os.Getenv("GOPATH")

// scionRoot is the root location of the scion infrastructure.
var DEF_SCIONDIR = path.Join(GOPATH, "src/github.com/scionproto/scion")

// ASConfig holds data about ia
type ASConfig struct {
	Sciond        string
	SdTomlPath    string
	TopologyPath  string
	MetricsServer string
}

// mapping AS addresses to their corresponding configs
type ASConfigs = map[string]ASConfig

// topology holds the IA from topology.json
type topology struct {
	IA string `json:"isd_as"`
}

type CmdOptions struct {
	Addr          string
	Port          int
	StaticRoot    string
	BrowseRoot    string
	ScionGen      string
	ScionGenCache string
}

func (o *CmdOptions) AbsPathCmdOptions() {
	o.StaticRoot, _ = filepath.Abs(o.StaticRoot)
	o.BrowseRoot, _ = filepath.Abs(o.BrowseRoot)
	o.ScionGen, _ = filepath.Abs(o.ScionGen)
	o.ScionGenCache, _ = filepath.Abs(o.ScionGenCache)
}

func isFlagUsed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func defaultStaticRoot() string {
	defaultWebappRoot := path.Join(os.Getenv("GOPATH"), "/src/github.com/netsec-ethz/scion-apps/webapp/")
	return path.Join(defaultWebappRoot, "web")
}

func defaultBrowseRoot(staticRoot string) string {
	return path.Join(staticRoot, "data")
}

func defaultScionGen() string {
	return "/etc/scion"
}

func defaultScionGenCache() string {
	return "/var/lib/scion"
}

func ParseFlags() CmdOptions {
	addr := flag.String(CMD_ADR, listenAddrDef, "Address of server host.")
	port := flag.Int(CMD_PRT, listenPortDef, "Port of server host.")
	staticRoot := flag.String(CMD_WEB, defaultStaticRoot(),
		"Path to read/write web server files.")
	browseRoot := flag.String(CMD_BRT, defaultBrowseRoot(*staticRoot),
		"Root path to read/browse from, CAUTION: read-access granted from -a and -p.")
	scionGen := flag.String(CMD_SCG, defaultScionGen(),
		"Path to read SCION gen directory of infrastructure config")
	scionGenCache := flag.String(CMD_SCC, defaultScionGenCache(),
		"Path to read SCION gen-cache directory of infrastructure run-time config")
	flag.Parse()
	// recompute root args to use the proper relative defaults if undefined
	if !isFlagUsed(CMD_WEB) {
		*staticRoot = defaultStaticRoot()
	}
	if !isFlagUsed(CMD_BRT) {
		*browseRoot = defaultBrowseRoot(*staticRoot)
	}
	if !isFlagUsed(CMD_SCG) {
		*scionGen = defaultScionGen()
	}
	if !isFlagUsed(CMD_SCC) {
		*scionGenCache = defaultScionGenCache()
	}
	options := CmdOptions{*addr, *port, *staticRoot, *browseRoot, *scionGen, *scionGenCache}
	options.AbsPathCmdOptions()
	return options
}

// ScanLocalSetting will load list of locally available IAs and their corresponding Scionds
func ScanLocalSetting(options *CmdOptions) ASConfigs {
	cfg := make(ASConfigs)
	var searchPath = options.ScionGen
	filepath.Walk(searchPath, func(path string, f os.FileInfo, _ error) error {
		if f != nil && f.Name() == topologyFile {
			dirPath := path[:len(path)-len(topologyFile)]
			// TODO sciond information is either sd.toml or sciond.toml, clean this once this is settled
			sdPath := dirPath + sdToml
			if _, err := os.Stat(sdPath); os.IsNotExist(err) {
				sdPath = dirPath + sciondToml
			}
			ia := getIAFromTopologyFile(path)
			cfg[ia] = ASConfig{
				Sciond:        getSDFromSDTomlFile(sdPath),
				SdTomlPath:    sdPath,
				TopologyPath:  path,
				MetricsServer: getMetricsServer(dirPath),
			}
		}
		return nil
	})
	return cfg
}

// getMetricsServer returns metrics server address from cs*.toml on the given path
func getMetricsServer(searchPath string) string {
	files, err := filepath.Glob(filepath.Join(searchPath, "cs*.toml"))
	if err != nil || len(files) != 1 {
		return ""
	}
	config, _ := toml.LoadFile(files[0])
	return config.Get("metrics.prometheus").(string) + "/metrics"
}

// getSDFromSDTomlFile returns sciond address from sd.toml on the given path
func getSDFromSDTomlFile(path string) string {
	if config, err := toml.LoadFile(path); err == nil {
		if sdAddr := config.Get("sd.address"); sdAddr != nil {
			return sdAddr.(string)
		}
	}
	log.Info(fmt.Sprintf("sciond address could not be read from toml file %s", path))
	if sd, ok := os.LookupEnv("SCION_DAEMON_ADDRESS"); ok {
		return sd
	}
	return sciond.DefaultAPIAddress
}

// getIAFromTopologyFile returns IA from topology.json on the given path
func getIAFromTopologyFile(path string) string {
	var t topology
	raw, _ := ioutil.ReadFile(path)
	json.Unmarshal([]byte(raw), &t)
	return t.IA
}

// Makes interfaces sortable, by preferred name
type byPrefInterface []net.Interface

func isInterfaceEn(c net.Interface) bool {
	return strings.HasPrefix(c.Name, "en")
}

func isInterfaceLocal(c net.Interface) bool {
	return strings.HasPrefix(c.Name, "lo")
}

func (c byPrefInterface) Len() int {
	return len(c)
}

func (c byPrefInterface) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c byPrefInterface) Less(i, j int) bool {
	// sort "en*" interfaces first, then "lo", then alphabetically
	if isInterfaceEn(c[i]) && !isInterfaceEn(c[j]) {
		return true
	}
	if !isInterfaceEn(c[i]) && isInterfaceEn(c[j]) {
		return false
	}
	if isInterfaceLocal(c[i]) && !isInterfaceLocal(c[j]) {
		return true
	}
	if !isInterfaceLocal(c[i]) && isInterfaceLocal(c[j]) {
		return false
	}
	return c[i].Name < c[j].Name
}

// GenServerNodeDefaults creates server defaults for localhost testing
func GenServerNodeDefaults(options *CmdOptions, localIAs []string) {
	// reverse sort so that the default server will oppose the default client
	sort.Sort(sort.Reverse(sort.StringSlice(localIAs)))

	serFp := path.Join(options.StaticRoot, cfgFileSerUser)
	jsonBuf := []byte(`{ "all": [`)
	for i := 0; i < len(localIAs); i++ {
		// use all localhost endpoints as possible servers for bwtester as least
		ia := strings.Replace(localIAs[i], "_", ":", -1)
		json := []byte(`{"name":"lo ` + ia + `","isdas":"` + ia +
			`", "addr":"` + serDefAddr + `","port":` + strconv.Itoa(serPortDef) +
			`}`)
		jsonBuf = append(jsonBuf, json...)
		if i < (len(localIAs) - 1) {
			jsonBuf = append(jsonBuf, []byte(`,`)...)
		}
	}
	jsonBuf = append(jsonBuf, []byte(`] }`)...)
	err := ioutil.WriteFile(serFp, jsonBuf, 0644)
	CheckError(err)
}

// GenClientNodeDefaults queries network interfaces and writes local client
// SCION addresses as json
func GenClientNodeDefaults(options *CmdOptions, cisdas string) {
	cliFp := path.Join(options.StaticRoot, cfgFileCliDef)

	// find interface addresses
	jsonBuf := []byte(`{ "all": [ `)
	ifaces, err := net.Interfaces()
	if CheckError(err) {
		return
	}
	sort.Sort(byPrefInterface(ifaces))
	idx := 0
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if CheckError(err) {
			continue
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				if ipnet.IP.To4() != nil {
					if idx > 0 {
						jsonBuf = append(jsonBuf, []byte(`, `)...)
					}
					cname := i.Name
					caddr := ipnet.IP.String()
					jsonInterface := []byte(`{"name":"` + cname + `", "isdas":"` +
						cisdas + `", "addr":"` + caddr + `","port":` +
						strconv.Itoa(cliPortDef) + `}`)
					jsonBuf = append(jsonBuf, jsonInterface...)
					idx++
				}
			}
		}
	}
	jsonBuf = append(jsonBuf, []byte(` ] }`)...)
	err = ioutil.WriteFile(cliFp, jsonBuf, 0644)
	CheckError(err)
}

// GetNodesHandler queries the local environment for user/default nodes.
func GetNodesHandler(w http.ResponseWriter, r *http.Request, options *CmdOptions) {
	r.ParseForm()
	nodes := r.PostFormValue("node_type")
	var fp string
	switch nodes {
	case "clients_default":
		fp = path.Join(options.StaticRoot, cfgFileCliDef)
	case "servers_default":
		fp = path.Join(options.StaticRoot, cfgFileSerDef)
	case "servers_user":
		fp = path.Join(options.StaticRoot, cfgFileSerUser)
	default:
		panic("Unhandled nodes type!")
	}
	raw, err := ioutil.ReadFile(fp)
	CheckError(err)
	fmt.Fprint(w, string(raw))
}
