// Copyright 2020 ETH Zurich
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

package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/drkey"
	"github.com/scionproto/scion/go/lib/drkey/protocol"
	"github.com/scionproto/scion/go/lib/sciond"
)

const (
	sciondForClient = "[fd00:f00d:cafe::7f00:b]:30255"
	sciondForServer = "127.0.0.19:30255"
)

// these next variables are also used as constants in the code
var timestamp = time.Now().UTC()
var srcIA, _ = addr.IAFromString("1-ff00:0:111")
var dstIA, _ = addr.IAFromString("1-ff00:0:112")
var srcHost = addr.HostFromIPStr("127.0.0.1")
var dstHost = addr.HostFromIPStr("fd00:f00d:cafe::7f00:a")

// Check just ensures the error is nil, or complains and quits
func check(e error) {
	if e != nil {
		panic(fmt.Sprintf("Fatal error: %v", e))
	}
}

type Client struct {
	sciond sciond.Connector
}

func NewClient(sciondPath string) Client {
	sciond, err := sciond.NewService(sciondPath).Connect(context.Background())
	check(err)
	return Client{
		sciond: sciond,
	}
}

func (c Client) HostKey(meta drkey.Lvl2Meta) drkey.Lvl2Key {
	ctx, cancelF := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelF()

	// get L2 key: (slow path)
	key, err := c.sciond.DRKeyGetLvl2Key(ctx, meta, timestamp)
	check(err)
	return key
}

func ThisClientAndMeta() (Client, drkey.Lvl2Meta) {
	c := NewClient(sciondForClient)
	meta := drkey.Lvl2Meta{
		KeyType:  drkey.Host2Host,
		Protocol: "piskes",
		SrcIA:    srcIA,
		DstIA:    dstIA,
		SrcHost:  srcHost,
		DstHost:  dstHost,
	}
	return c, meta
}

type Server struct {
	sciond sciond.Connector
}

func NewServer(sciondPath string) Server {
	sciond, err := sciond.NewService(sciondPath).Connect(context.Background())
	check(err)
	return Server{
		sciond: sciond,
	}
}

func (s Server) dsForServer(meta drkey.Lvl2Meta) drkey.DelegationSecret {
	ctx, cancelF := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelF()

	dsMeta := drkey.Lvl2Meta{
		KeyType:  drkey.AS2AS,
		Protocol: meta.Protocol,
		SrcIA:    meta.SrcIA,
		DstIA:    meta.DstIA,
	}
	lvl2Key, err := s.sciond.DRKeyGetLvl2Key(ctx, dsMeta, timestamp)
	check(err)
	fmt.Printf("Only the server obtains it: DS key = %s\n", hex.EncodeToString(lvl2Key.Key))
	ds := drkey.DelegationSecret{
		Protocol: lvl2Key.Protocol,
		Epoch:    lvl2Key.Epoch,
		SrcIA:    lvl2Key.SrcIA,
		DstIA:    lvl2Key.DstIA,
		Key:      lvl2Key.Key,
	}
	return ds
}

func (s Server) HostKeyFromDS(meta drkey.Lvl2Meta, ds drkey.DelegationSecret) drkey.Lvl2Key {
	piskes := (protocol.KnownDerivations["piskes"]).(protocol.DelegatedDerivation)
	derived, err := piskes.DeriveLvl2FromDS(meta, ds)
	check(err)
	return derived
}

func ThisServerAndMeta() (Server, drkey.Lvl2Meta) {
	server := NewServer(sciondForServer)
	meta := drkey.Lvl2Meta{
		KeyType:  drkey.Host2Host,
		Protocol: "piskes",
		SrcIA:    srcIA,
		DstIA:    dstIA,
		SrcHost:  srcHost,
		DstHost:  dstHost,
	}
	return server, meta
}

func main() {
	var clientKey, serverKey drkey.Lvl2Key

	client, metaClient := ThisClientAndMeta()
	t0 := time.Now()
	clientKey = client.HostKey(metaClient)
	durationClient := time.Since(t0)

	server, metaServer := ThisServerAndMeta()
	ds := server.dsForServer(metaServer)
	t0 = time.Now()
	serverKey = server.HostKeyFromDS(metaServer, ds)
	durationServer := time.Since(t0)

	fmt.Printf("Client,\thost key = %s\tduration = %s\n", hex.EncodeToString(clientKey.Key), durationClient)
	fmt.Printf("Server,\thost key = %s\tduration = %s\n", hex.EncodeToString(serverKey.Key), durationServer)
}
