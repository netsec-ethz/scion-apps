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

package scionutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
)

const localhost = "localhost"

// InitSCION initializes the default SCION networking context with the provided SCION address
// and the default SCIOND/SCION dispatcher
func InitSCION(localAddr *snet.Addr) error {
	err := snet.Init(localAddr.IA, GetSCIONDPath(&localAddr.IA), GetDefaultDispatcher())
	if err != nil {
		return err
	}
	return nil
}

// InitSCIONString initializes the default SCION networking context with provided SCION address in string format
// and the default SCIOND/SCION dispatcher
func InitSCIONString(localAddr string) (*snet.Addr, error) {
	addr, err := snet.AddrFromString(localAddr)
	if err != nil {
		return nil, err
	}

	return addr, InitSCION(addr)
}

// GetSCIONDPath returns the path to the SCION socket.
func GetSCIONDPath(ia *addr.IA) string {

	// Use default.sock if exists:
	if _, err := os.Stat(sciond.DefaultSCIONDPath); err == nil {
		return sciond.DefaultSCIONDPath
	}
	// otherwise, use socket with ia name:
	return sciond.GetDefaultSCIONDPath(ia)
}

// GetDefaultDispatcher returns the path to the default SCION dispatcher
func GetDefaultDispatcher() reliable.DispatcherService {
	return reliable.NewDispatcherService("")
}

// GetLocalhost returns a local SCION address an application can bind to
func GetLocalhost() (*snet.Addr, error) {
	str, err := GetLocalhostString()
	if err != nil {
		return nil, err
	}
	addr, err := snet.AddrFromString(str)
	if err != nil {
		return nil, err
	}
	return addr, nil
}

// GetLocalhostString returns a local SCION address an application can bind to
func GetLocalhostString() (string, error) {

	var ia addr.IA
	var l3 addr.HostAddr
	var err error

	// see if 'localhost' is defined in hostsfile
	ia, l3, err = GetHostByName(localhost)
	if err == nil {
		return fmt.Sprintf("%s,[%s]", ia, l3), nil
	}

	// otherwise return ISD-AS and loopback IP
	sc := os.Getenv("SC")
	b, err := ioutil.ReadFile(filepath.Join(sc, "gen/ia"))
	if err != nil {
		return "", err
	}
	ia, err = addr.IAFromFileFmt(string(b), false)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s,[127.0.0.1]", ia), nil
}
