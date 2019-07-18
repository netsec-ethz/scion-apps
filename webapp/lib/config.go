package lib

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	log "github.com/inconshreveable/log15"
	. "github.com/netsec-ethz/scion-apps/webapp/util"
)

// SCIONROOT is the root location of the scion infrastructure.
var SCIONROOT = "src/github.com/scionproto/scion"

// LABROOT is the root location of scionlab apps.
var LABROOT = "src/github.com/netsec-ethz/scion-apps"

// GOPATH is the root of the GOPATH environment.
var GOPATH = os.Getenv("GOPATH")

// default params for localhost testing
var serIaDef = "1-ff00:0:112"
var cliPortDef = "30001"
var serPortDefBwt = "30100"
var serPortDefImg = "42002"
var serPortDefSen = "42003"
var serDefAddr = "127.0.0.2"

var cfgFileCliUser = "config/clients_user.json"
var cfgFileSerUser = "config/servers_user.json"
var cfgFileCliDef = "config/clients_default.json"
var cfgFileSerDef = "config/servers_default.json"

// UserSetting holds the serialized structure for persistent user settings
type UserSetting struct {
	MyIA string `json:"myIa"`
}

// WriteUserSetting writes the settings to disk.
func WriteUserSetting(srcpath string, settings UserSetting) {
	cliUserFp := path.Join(srcpath, cfgFileCliUser)
	settingsJSON, _ := json.Marshal(settings)

	err := ioutil.WriteFile(cliUserFp, settingsJSON, 0644)
	CheckError(err)
}

// ReadUserSetting reads the settings from disk.
func ReadUserSetting(srcpath string) UserSetting {
	var settings UserSetting
	cliUserFp := path.Join(srcpath, cfgFileCliUser)

	// no problem when user settings not set yet
	raw, err := ioutil.ReadFile(cliUserFp)
	log.Debug("ReadClientsUser", "settings", string(raw))
	if !CheckError(err) {
		json.Unmarshal([]byte(raw), &settings)
	}
	return settings
}

// ScanLocalIAs will load list of locally available IAs
func ScanLocalIAs() []string {
	var localIAs []string
	var reIaFilePathCap = `\/ISD([0-9]+)\/AS(\w+)`
	re := regexp.MustCompile(reIaFilePathCap)
	var searchPath = path.Join(GOPATH, SCIONROOT, "gen")
	filepath.Walk(searchPath, func(path string, f os.FileInfo, _ error) error {
		if f != nil && f.IsDir() {
			capture := re.FindStringSubmatch(path)
			if len(capture) > 0 {
				ia := capture[1] + "-" + capture[2]
				ia = strings.Replace(ia, "_", ":", -1) // convert once
				if !StringInSlice(localIAs, ia) {
					log.Debug("Local IA Found:", "ia", ia)
					localIAs = append(localIAs, ia)
				}
			}
		}
		return nil
	})
	sort.Strings(localIAs)
	return localIAs
}

// StringInSlice can check a slice for a unique string
func StringInSlice(arr []string, i string) bool {
	for _, v := range arr {
		if v == i {
			return true
		}
	}
	return false
}

// Makes interfaces sortable, by preferred name
type byPrefInterface []net.Interface

func isInterfaceEnp(c net.Interface) bool {
	return strings.HasPrefix(c.Name, "enp")
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
	// sort "enp" interfaces first, then "lo", then alphabetically
	if isInterfaceEnp(c[i]) && !isInterfaceEnp(c[j]) {
		return true
	}
	if !isInterfaceEnp(c[i]) && isInterfaceEnp(c[j]) {
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
func GenServerNodeDefaults(srcpath string, localIAs []string) {
	// reverse sort so that the default server will oppose the default client
	sort.Sort(sort.Reverse(sort.StringSlice(localIAs)))

	serFp := path.Join(srcpath, cfgFileSerUser)
	jsonBuf := []byte(`{ "all": [`)
	for i := 0; i < len(localIAs); i++ {
		// use all localhost endpoints as possible servers for bwtester as least
		ia := strings.Replace(localIAs[i], "_", ":", -1)
		json := []byte(`{"name":"lo ` + ia + `","isdas":"` + ia +
			`", "addr":"` + serDefAddr + `","port":` + serPortDefBwt + `}`)
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
func GenClientNodeDefaults(srcpath, cisdas string) {
	cliFp := path.Join(srcpath, cfgFileCliDef)
	cport := cliPortDef

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
						cisdas + `", "addr":"` + caddr + `","port":` + cport + `}`)
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
func GetNodesHandler(w http.ResponseWriter, r *http.Request, srcpath string) {
	r.ParseForm()
	nodes := r.PostFormValue("node_type")
	var fp string
	switch nodes {
	case "clients_default":
		fp = path.Join(srcpath, cfgFileCliDef)
	case "servers_default":
		fp = path.Join(srcpath, cfgFileSerDef)
	case "clients_user":
		fp = path.Join(srcpath, cfgFileCliUser)
	case "servers_user":
		fp = path.Join(srcpath, cfgFileSerUser)
	default:
		panic("Unhandled nodes type!")
	}
	raw, err := ioutil.ReadFile(fp)
	CheckError(err)
	fmt.Fprintf(w, string(raw))
}
