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
	"fmt"
	"time"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/drkey"
	"github.com/scionproto/scion/pkg/experimental/fabrid/crypto"
	"github.com/scionproto/scion/pkg/private/serrors"
	"github.com/scionproto/scion/pkg/slayers/extension"
)

type FabridServer struct {
	Local     UDPAddr
	Source    UDPAddr
	tmpBuffer []byte
	pathKey   *drkey.HostHostKey
	ctx       context.Context
}

func NewFabridServer(ctx context.Context, local UDPAddr, remote UDPAddr) *FabridServer {
	server := &FabridServer{
		Local:     local,
		Source:    remote,
		tmpBuffer: make([]byte, 192),
		ctx:       ctx,
	}
	err := server.refreshPathKey(time.Now())
	if err != nil {
		fmt.Println("Failed to fetch path key. Does your local IP differ from your SD?\nError:", err)
		return nil
	}
	return server
}

func (s *FabridServer) refreshPathKey(validity time.Time) error {
	if s.pathKey == nil || !s.pathKey.Epoch.Contains(validity) {
		meta := drkey.HostHostMeta{
			Validity: validity,
			SrcIA:    addr.IA(s.Local.IA),
			SrcHost:  s.Local.IP.String(),
			DstIA:    addr.IA(s.Source.IA),
			DstHost:  s.Source.IP.String(),
			ProtoId:  drkey.FABRID,
		}
		hostHostKey, err := host().drkeyGetHostHostKey(s.ctx, meta)
		if err != nil {
			return serrors.WrapStr("getting host key", err)
		}
		s.pathKey = &hostHostKey

	}
	return nil
}

func (s *FabridServer) HandleFabridPacket(fabridOption *extension.FabridOption,
	identifierOption *extension.IdentifierOption) error {
	err := s.refreshPathKey(identifierOption.Timestamp)
	if err != nil {
		fmt.Println("Failed to fetch path key. Does your local IP differ from your SD?\nError:", err)
		return err
	}
	_, _, err = crypto.VerifyPathValidator(fabridOption,
		s.tmpBuffer, s.pathKey.Key[:])
	if err != nil {
		fmt.Println("Failed to verify FABRID packet. Error:", err)
		return err
	}
	return nil
}
