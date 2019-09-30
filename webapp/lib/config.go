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

type CmdOptions struct {
	StaticRoot, BrowseRoot, AppsRoot, ScionRoot, ScionBin, ScionGen, ScionGenCache, ScionLogs string
}

// WriteUserSetting writes the settings to disk.
func WriteUserSetting(options *CmdOptions, settings UserSetting) {
	cliUserFp := path.Join(options.StaticRoot, cfgFileCliUser)
	settingsJSON, _ := json.Marshal(settings)

	err := ioutil.WriteFile(cliUserFp, settingsJSON, 0644)
	CheckError(err)
}

// ReadUserSetting reads the settings from disk.
func ReadUserSetting(options *CmdOptions) UserSetting {
	var settings UserSetting
	cliUserFp := path.Join(options.StaticRoot, cfgFileCliUser)

	// no problem when user settings not set yet
	raw, _ := ioutil.ReadFile(cliUserFp)
	log.Debug("ReadUserSetting from saved", "settings", string(raw))
	json.Unmarshal([]byte(raw), &settings)

	return settings
}

// ScanLocalIAs will load list of locally available IAs
func ScanLocalIAs(options *CmdOptions) []string {
	var localIAs []string
	var reIaFilePathCap = `\/ISD([0-9]+)\/AS(\w+)`
	re := regexp.MustCompile(reIaFilePathCap)
	var searchPath = options.ScionGen
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
func GenServerNodeDefaults(options *CmdOptions, localIAs []string) {
	// reverse sort so that the default server will oppose the default client
	sort.Sort(sort.Reverse(sort.StringSlice(localIAs)))

	serFp := path.Join(options.StaticRoot, cfgFileSerUser)
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
func GenClientNodeDefaults(options *CmdOptions, cisdas string) {
	cliFp := path.Join(options.StaticRoot, cfgFileCliDef)
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
func GetNodesHandler(w http.ResponseWriter, r *http.Request, options *CmdOptions) {
	r.ParseForm()
	nodes := r.PostFormValue("node_type")
	var fp string
	switch nodes {
	case "clients_default":
		fp = path.Join(options.StaticRoot, cfgFileCliDef)
	case "servers_default":
		fp = path.Join(options.StaticRoot, cfgFileSerDef)
	case "clients_user":
		fp = path.Join(options.StaticRoot, cfgFileCliUser)
	case "servers_user":
		fp = path.Join(options.StaticRoot, cfgFileSerUser)
	default:
		panic("Unhandled nodes type!")
	}
	raw, err := ioutil.ReadFile(fp)
	CheckError(err)
	fmt.Fprintf(w, string(raw))
}
