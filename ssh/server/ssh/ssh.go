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
	"fmt"
	"net"
	"os"
	"strconv"

	log "github.com/inconshreveable/log15"
	"golang.org/x/crypto/ssh"

	"github.com/netsec-ethz/scion-apps/ssh/server/serverconfig"
	"github.com/netsec-ethz/scion-apps/ssh/utils"
)

// ChannelHandlerFunction is a type for channel handlers, such as terminal sessions, tunnels, or X11 forwarding.
type channelHandlerFunction func(perms *ssh.Permissions, newChannel ssh.NewChannel) error

// Server is a struct containing information about SSH servers.
type Server struct {
	authorizedKeysFile string

	configuration *ssh.ServerConfig

	channelHandlers map[string]channelHandlerFunction
}

// Create creates a new unconnected Server object.
func Create(config *serverconfig.ServerConfig, version string) (*Server, error) {
	server := &Server{
		authorizedKeysFile: config.AuthorizedKeysFile,
		channelHandlers:    make(map[string]channelHandlerFunction),
	}

	maxAuthTries, _ := strconv.Atoi(config.MaxAuthTries)
	server.configuration = &ssh.ServerConfig{
		PasswordCallback:  server.PasswordAuth,
		PublicKeyCallback: server.PublicKeyAuth,
		MaxAuthTries:      maxAuthTries,
		//ServerVersion: fmt.Sprintf("SCION-ssh-server-v%s", version),
	}

	privateBytes, err := os.ReadFile(utils.ParsePath(config.HostKey))
	if err != nil {
		return nil, fmt.Errorf("failed loading private key: %w", err)
	}
	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		return nil, fmt.Errorf("failed parsing private key: %w", err)
	}
	server.configuration.AddHostKey(private)

	server.channelHandlers["session"] = handleSession
	server.channelHandlers["direct-tcpip"] = handleTCPTunnel
	server.channelHandlers["direct-scionquic"] = handleSCIONQUICTunnel

	return server, nil
}

func (s *Server) handleChannels(perms *ssh.Permissions, chans <-chan ssh.NewChannel) {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		go s.handleChannel(perms, newChannel)
	}
}

func (s *Server) handleChannel(perms *ssh.Permissions, newChannel ssh.NewChannel) {
	if handler, exists := s.channelHandlers[newChannel.ChannelType()]; exists {
		err := handler(perms, newChannel)
		if err != nil {
			log.Error("error handling channel", "type", newChannel.ChannelType(), "err", err)
		}
	} else {
		_ = newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", newChannel.ChannelType()))
		return
	}
}

// HandleConnection handles a client connection.
func (s *Server) HandleConnection(conn net.Conn) {
	log.Debug("Handling new connection")
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.configuration)
	if err != nil {
		log.Error("Failed to create new connection", "error", err)
		conn.Close()
	}

	log.Debug("New SSH connection", "remoteAddress", sshConn.RemoteAddr(), "clientVersion", sshConn.ClientVersion())
	// Discard all global out-of-band Requests
	go ssh.DiscardRequests(reqs)
	// Accept all channels
	s.handleChannels(sshConn.Permissions, chans)
}
