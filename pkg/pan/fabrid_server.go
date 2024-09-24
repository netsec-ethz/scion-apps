// Copyright 2021 ETH Zurich
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

package pan

import (
	"context"
	"time"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/drkey"
	"github.com/scionproto/scion/pkg/experimental/fabrid/crypto"
	"github.com/scionproto/scion/pkg/log"
	"github.com/scionproto/scion/pkg/private/serrors"
	"github.com/scionproto/scion/pkg/slayers/extension"
)

type ClientConnection struct {
	Source    UDPAddr
	tmpBuffer []byte
	pathKey   drkey.Key
}

type FabridServer struct {
	Local       UDPAddr
	Connections map[string]*ClientConnection
	ASKeyCache  map[addr.IA]drkey.HostASKey
}

func NewFabridServer(local *UDPAddr) *FabridServer {
	server := &FabridServer{
		Local:       *local,
		Connections: make(map[string]*ClientConnection),
		ASKeyCache:  make(map[addr.IA]drkey.HostASKey),
	}
	return server
}

func (s *FabridServer) FetchHostHostKey(dstHost UDPAddr,
	validity time.Time) (drkey.Key, error) {
	meta := drkey.HostHostMeta{
		Validity: validity,
		SrcIA:    addr.IA(s.Local.IA),
		SrcHost:  s.Local.IP.String(),
		DstIA:    addr.IA(dstHost.IA),
		DstHost:  dstHost.IP.String(),
		ProtoId:  drkey.FABRID,
	}
	hostHostKey, err := GetDRKeyHostHostKey(context.Background(), meta)
	if err != nil {
		return drkey.Key{}, serrors.WrapStr("getting host key", err)
	}
	return hostHostKey.Key, nil
}

func (s *FabridServer) HandleFabridPacket(remote UDPAddr, fabridOption *extension.FabridOption,
	identifierOption *extension.IdentifierOption) error {
	client, found := s.Connections[remote.String()]
	if !found {
		pathKey, err := s.FetchHostHostKey(remote, identifierOption.Timestamp)
		if err != nil {
			return err
		}
		client = &ClientConnection{
			Source:    remote,
			tmpBuffer: make([]byte, 192),
			pathKey:   pathKey,
		}
		s.Connections[remote.String()] = client
		log.Info("Opened new connection", "remote", remote.String())
	}

	_, err := crypto.VerifyPathValidator(fabridOption,
		client.tmpBuffer, client.pathKey[:])
	if err != nil {
		return err
	}
	return nil
}
