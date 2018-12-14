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
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
)

// InitSCION initializes the default SCION networking context with the provided SCION address
// and the default SCIOND/SCION dispatcher
func InitSCION(localAddr *snet.Addr) error {
	err := snet.Init(localAddr.IA, GetDefaultSCIOND(), GetDefaultDispatcher())
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

	err = snet.Init(addr.IA, GetDefaultSCIOND(), GetDefaultDispatcher())
	if err != nil {
		return nil, err
	}

	return addr, nil
}

// GetDefaultSCIOND returns the path to the default SCION socket
func GetDefaultSCIOND() string {
	return sciond.GetDefaultSCIONDPath(nil)
}

// GetDefaultDispatcher returns the path to the default SCION dispatcher
func GetDefaultDispatcher() string {
	return "/run/shm/dispatcher/default.sock"
}
