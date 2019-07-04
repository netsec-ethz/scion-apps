package lib

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

	. "github.com/netsec-ethz/scion-apps/webapp/util"
)

// SCIONROOT is the root location of the scion infrastructure.
var SCIONROOT = "src/github.com/scionproto/scion"

// LABROOT is the root location of scionlab apps.
var LABROOT = "src/github.com/netsec-ethz/scion-apps"

// GOPATH is the root of the GOPATH environment.
var GOPATH = os.Getenv("GOPATH")

// default params for localhost testing
var cliIaDef = "1-ff00:0:111"
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

// GetLocalIa reads locally generated file for this IA's name, if written
func GetLocalIa() string {
	filepath := path.Join(GOPATH, SCIONROOT, "gen/ia")
	b, err := ioutil.ReadFile(filepath)
	if CheckError(err) {
		return ""
	}
	return strings.Replace(strings.TrimSpace(string(b)), "_", ":", -1)
}

func GetCliIaDef() string {
	return cliIaDef
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
func GenServerNodeDefaults(srcpath string) {
	serFp := path.Join(srcpath, cfgFileSerUser)
	jsonBuf := []byte(`{ `)
	json := []byte(`"bwtester": [{"name":"lo ` + serIaDef + `","isdas":"` +
		serIaDef + `", "addr":"` + serDefAddr + `","port":` + serPortDefBwt +
		`},{"name":"lo 2-ff00:0:222","isdas":"2-ff00:0:222", "addr":"127.0.0.22","port":30101}], `)
	jsonBuf = append(jsonBuf, json...)
	json = []byte(`"camerapp": [{"name":"localhost","isdas":"` +
		serIaDef + `", "addr":"` + serDefAddr + `","port":` + serPortDefImg + `}], `)
	jsonBuf = append(jsonBuf, json...)
	json = []byte(`"sensorapp": [{"name":"localhost","isdas":"` +
		serIaDef + `", "addr":"` + serDefAddr + `","port":` + serPortDefSen + `}], `)
	jsonBuf = append(jsonBuf, json...)
	json = []byte(`"echo": [{"name":"localhost","isdas":"` +
		serIaDef + `", "addr":"` + serDefAddr + `","port":` + serPortDefSen + `}], `)
	jsonBuf = append(jsonBuf, json...)
	json = []byte(`"traceroute": [{"name":"localhost","isdas":"` +
		serIaDef + `", "addr":"` + serDefAddr + `","port":` + serPortDefSen + `}]`)
	jsonBuf = append(jsonBuf, json...)

	jsonBuf = append(jsonBuf, []byte(` }`)...)
	err := ioutil.WriteFile(serFp, jsonBuf, 0644)
	CheckError(err)
}

// GenClientNodeDefaults queries network interfaces and writes local client
// SCION addresses as json
func GenClientNodeDefaults(srcpath string) {
	cliFp := path.Join(srcpath, cfgFileCliDef)
	cisdas := GetLocalIa()
	if len(cisdas) == 0 {
		cisdas = cliIaDef
	}
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
