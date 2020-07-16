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
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	log "github.com/inconshreveable/log15"

	"github.com/netsec-ethz/scion-apps/ssh/quicconn"
	"golang.org/x/crypto/ssh"
)

func handleTunnelForRemoteConnection(connection ssh.Channel, remoteConnection net.Conn) {
	// Prepare teardown function
	close := func() {
		connection.Close()
		remoteConnection.Close()
	}

	var once sync.Once
	go func() {
		io.Copy(connection, remoteConnection)
		once.Do(close)
	}()
	go func() {
		io.Copy(remoteConnection, connection)
		once.Do(close)
	}()
}

func handleTCPTunnel(perms *ssh.Permissions, newChannel ssh.NewChannel) {
	extraData := newChannel.ExtraData()
	addressLen := binary.BigEndian.Uint32(extraData[0:4])
	address := string(extraData[4 : addressLen+4])
	port := binary.BigEndian.Uint32(extraData[addressLen+4 : addressLen+8])

	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Debug("Could not accept channel (%s)", err)
		return
	}

	go ssh.DiscardRequests(requests)

	remoteConnection, err := net.Dial("tcp", fmt.Sprintf("%s:%v", address, port))
	if err != nil {
		log.Debug("Could not open remote connection (%s)", err)
		return
	}

	handleTunnelForRemoteConnection(connection, remoteConnection)
}

func handleSCIONQUICTunnel(perms *ssh.Permissions, newChannel ssh.NewChannel) {
	extraData := newChannel.ExtraData()
	addressLen := binary.BigEndian.Uint32(extraData[0:4])
	address := string(extraData[4 : addressLen+4])

	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Debug("Could not accept channel (%s)", err)
		return
	}

	go ssh.DiscardRequests(requests)

	remoteConnection, err := quicconn.Dial(address)
	if err != nil {
		log.Debug("Could not open remote connection (%s)", err)
		return
	}

	handleTunnelForRemoteConnection(connection, remoteConnection)
}
