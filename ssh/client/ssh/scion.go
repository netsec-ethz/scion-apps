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

package ssh

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/quic-go/quic-go"
	"golang.org/x/crypto/ssh"
	"inet.af/netaddr"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
)

// dialSCION starts a client connection to the given SSH server over SCION using QUIC.
func dialSCION(ctx context.Context,
	addr string,
	policy pan.Policy,
	selector string,
	config *ssh.ClientConfig) (*ssh.Client, error) {

	remote, err := pan.ResolveUDPAddr(ctx, addr)
	if err != nil {
		return nil, err
	}
	sel, err := selectorByName(selector)
	if err != nil {
		return nil, err
	}
	tlsConf := &tls.Config{
		NextProtos:         []string{quicutil.SingleStreamProto},
		InsecureSkipVerify: true,
	}
	quicConf := &quic.Config{
		KeepAlivePeriod: time.Duration(15) * time.Second,
	}
	sess, err := pan.DialQUIC(ctx, netaddr.IPPort{}, remote, policy, sel, "", tlsConf, quicConf)
	if err != nil {
		return nil, err
	}
	stream, err := quicutil.NewSingleStream(sess)
	if err != nil {
		return nil, err
	}
	conn, nc, rc, err := ssh.NewClientConn(stream, addr, config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(conn, nc, rc), nil
}

// tunnelDialSCION creates a tunnel using the given SSH client.
func tunnelDialSCION(client *ssh.Client, addr string) (ssh.Channel, error) {
	openChannelData := directSCIONData{
		addr: addr,
	}

	c, requests, err := client.OpenChannel("direct-scionquic", ssh.Marshal(&openChannelData))
	if err != nil {
		return nil, err
	}

	go ssh.DiscardRequests(requests)
	return c, nil
}

type directSCIONData struct {
	addr string
}
