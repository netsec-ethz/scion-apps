package ssh

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	log "github.com/inconshreveable/log15"

	"github.com/netsec-ethz/scion-apps/ssh/scionutils"
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

	remoteConnection, err := scionutils.DialSCION("", address)
	if err != nil {
		log.Debug("Could not open remote connection (%s)", err)
		return
	}

	handleTunnelForRemoteConnection(connection, remoteConnection)
}
