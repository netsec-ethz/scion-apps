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
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"os"
)

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

	err = snet.Init(addr.IA, GetSCIONDPath(&addr.IA), GetDefaultDispatcher())
	if err != nil {
		return nil, err
	}

	return addr, nil
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
func GetDefaultDispatcher() string {
	return "/run/shm/dispatcher/default.sock"
}
