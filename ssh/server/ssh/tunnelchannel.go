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
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
)

func handleTunnelForRemoteConnection(connection ssh.Channel, remoteConnection net.Conn) {
	// Prepare teardown function
	close := func() {
		connection.Close()
		remoteConnection.Close()
	}

	var once sync.Once
	go func() {
		_, _ = io.Copy(connection, remoteConnection)
		once.Do(close)
	}()
	go func() {
		_, _ = io.Copy(remoteConnection, connection)
		once.Do(close)
	}()
}

func handleTCPTunnel(perms *ssh.Permissions, newChannel ssh.NewChannel) error {
	extraData := newChannel.ExtraData()
	addressLen := binary.BigEndian.Uint32(extraData[0:4])
	address := string(extraData[4 : addressLen+4])
	port := binary.BigEndian.Uint32(extraData[addressLen+4 : addressLen+8])

	connection, requests, err := newChannel.Accept()
	if err != nil {
		return fmt.Errorf("could not accept channel: %w", err)
	}

	go ssh.DiscardRequests(requests)

	remoteConnection, err := net.Dial("tcp", fmt.Sprintf("%s:%v", address, port))
	if err != nil {
		return fmt.Errorf("could not open remote connection: %w", err)
	}

	handleTunnelForRemoteConnection(connection, remoteConnection)
	return nil
}

func handleSCIONQUICTunnel(perms *ssh.Permissions, newChannel ssh.NewChannel) error {
	extraData := newChannel.ExtraData()
	addressLen := binary.BigEndian.Uint32(extraData[0:4])
	address := string(extraData[4 : addressLen+4])

	connection, requests, err := newChannel.Accept()
	if err != nil {
		return fmt.Errorf("could not accept channel: %w", err)
	}

	go ssh.DiscardRequests(requests)

	ctx := context.Background()
	remote, err := pan.ResolveUDPAddr(context.TODO(), address)
	if err != nil {
		return fmt.Errorf("could not resolve remote address: %w", err)
	}
	tlsConf := &tls.Config{
		NextProtos:         []string{quicutil.SingleStreamProto},
		InsecureSkipVerify: true,
	}
	sess, err := pan.DialQUIC(ctx, netip.AddrPort{}, remote, nil, nil, nil, "", tlsConf, nil)
	if err != nil {
		return fmt.Errorf("could not open remote connection: %w", err)
	}
	remoteConnection, err := quicutil.NewSingleStream(sess)
	if err != nil {
		return err
	}

	handleTunnelForRemoteConnection(connection, remoteConnection)
	return nil
}
