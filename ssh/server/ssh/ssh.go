package ssh

import (
	"fmt"
	"io/ioutil"
	"net"
	"strconv"

	log "github.com/inconshreveable/log15"

	"golang.org/x/crypto/ssh"

	"github.com/netsec-ethz/scion-apps/ssh/server/serverconfig"
	"github.com/netsec-ethz/scion-apps/ssh/utils"
)

type ChannelHandlerFunction func(newChannel ssh.NewChannel)

type SSHServer struct {
	authorizedKeysFile string

	configuration *ssh.ServerConfig

	channelHandlers map[string]ChannelHandlerFunction
}

func Create(config *serverconfig.ServerConfig, version string) (*SSHServer, error) {
	server := &SSHServer{
		authorizedKeysFile: config.AuthorizedKeysFile,
		channelHandlers:    make(map[string]ChannelHandlerFunction),
	}

	maxAuthTries, _ := strconv.Atoi(config.MaxAuthTries)
	server.configuration = &ssh.ServerConfig{
		PasswordCallback:  server.PasswordAuth,
		PublicKeyCallback: server.PublicKeyAuth,
		MaxAuthTries:      maxAuthTries,
		//ServerVersion: fmt.Sprintf("SCION-ssh-server-v%s", version),
	}

	privateBytes, err := ioutil.ReadFile(utils.ParsePath(config.HostKey))
	if err != nil {
		return nil, fmt.Errorf("Failed loading private key: %v", err)
	}
	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		return nil, fmt.Errorf("Failed parsing private key: %v", err)
	}
	server.configuration.AddHostKey(private)

	server.channelHandlers["session"] = handleSession
	server.channelHandlers["direct-tcpip"] = handleTCPTunnel
	server.channelHandlers["direct-scionquic"] = handleSCIONQUICTunnel

	return server, nil
}

func (s *SSHServer) handleChannels(chans <-chan ssh.NewChannel) {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		go s.handleChannel(newChannel)
	}
}

func (s *SSHServer) handleChannel(newChannel ssh.NewChannel) {
	if handler, exists := s.channelHandlers[newChannel.ChannelType()]; exists {
		handler(newChannel)
	} else {
		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", newChannel.ChannelType()))
		return
	}
}

func (s *SSHServer) HandleConnection(conn net.Conn) error {
	log.Debug("Handling new connection")
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.configuration)
	if err != nil {
		log.Debug("Failed to create new connection (%s)", err)
		conn.Close()
		return err
	}

	log.Debug("New SSH connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())
	// Discard all global out-of-band Requests
	go ssh.DiscardRequests(reqs)
	// Accept all channels
	s.handleChannels(chans)

	return nil
}
