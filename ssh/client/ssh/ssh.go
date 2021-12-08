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
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	log "github.com/inconshreveable/log15"
	"golang.org/x/crypto/ssh"
	"inet.af/netaddr"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
	"github.com/netsec-ethz/scion-apps/ssh/client/clientconfig"
	"github.com/netsec-ethz/scion-apps/ssh/client/ssh/knownhosts"
	"github.com/netsec-ethz/scion-apps/ssh/utils"
)

// AuthenticationHandler is a function that represents an authentication method.
type AuthenticationHandler func() (secret string, err error)

// VerifyHostKeyHandler is a function that verifies host keys, often by user interaction
type VerifyHostKeyHandler func(hostname string, remote net.Addr, key string) bool

// Client is a struct representing an SSH client. It consists of a connection to the server, and at most one terminal session (per SSH specification, a connection may not serve multiple sessions)
type Client struct {
	config                          *ssh.ClientConfig
	promptForForeignKeyConfirmation VerifyHostKeyHandler
	knownHostsFileHandler           ssh.HostKeyCallback
	knownHostsFilePath              string

	client  *ssh.Client
	session *ssh.Session
}

// Create creates a new unconnected Client.
func Create(username string, config *clientconfig.ClientConfig, passAuthHandler AuthenticationHandler,
	verifyNewKeyHandler VerifyHostKeyHandler) (*Client, error) {
	client := &Client{
		config: &ssh.ClientConfig{
			User: username,
		},
	}

	var authMethods []ssh.AuthMethod

	// Load client private key
	if config.PubkeyAuthentication == "yes" {
		for i := len(config.IdentityFile) - 1; i >= 0; i-- {
			am, err := loadPrivateKey(utils.ParsePath(config.IdentityFile[i]))
			if err != nil {
				log.Debug("Error loading private key, skipped.", "IdentityFile", config.IdentityFile[i], "err", err)
			} else {
				log.Debug("Loaded private key", "IdentityFile", config.IdentityFile[i])
				authMethods = append(authMethods, am)
			}
		}
	}

	// Use password auth
	if config.PasswordAuthentication == "yes" {
		log.Debug("Configuring password auth")
		authMethods = append(authMethods, ssh.PasswordCallback(passAuthHandler))
	}

	if config.StrictHostKeyChecking != "no" {
		knownHostsFile := utils.ParsePath(config.UserKnownHostsFile)
		// Create file if doesn't exist
		if _, err := os.Stat(knownHostsFile); os.IsNotExist(err) {
			var file, err = os.Create(knownHostsFile)
			if err != nil {
				return nil, err
			}
			file.Close()
		}

		client.knownHostsFilePath = knownHostsFile
		khh, err := knownhosts.New(knownHostsFile)
		if err != nil {
			return nil, err
		}
		client.knownHostsFileHandler = khh
		client.config.HostKeyCallback = client.verifyHostKey
		client.promptForForeignKeyConfirmation = verifyNewKeyHandler
	} else {
		log.Debug("Not verifying host key!")
		client.config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}
	client.config.Auth = authMethods

	return client, nil
}

// Connect connects the Client to the given address.
func (client *Client) Connect(ctx context.Context, addr string, policy pan.Policy, selector string) error {
	goClient, err := dialSCION(ctx, addr, policy, selector, client.config)
	if err != nil {
		return err
	}

	client.client = goClient

	client.session, err = client.client.NewSession()
	if err != nil {
		return err
	}

	return nil
}

// RunSession runs a terminal session, waiting for it to end.
func (client *Client) RunSession(cmd string) error {
	return client.session.Run(cmd)
}

// StartSession starts a terminal session, not waiting for it to end.
func (client *Client) StartSession(cmd string) error {
	return client.session.Start(cmd)
}

// WaitSession waits for a terminal session to end.
func (client *Client) WaitSession() error {
	return client.session.Wait()
}

// ConnectPipes connects the given reader and writer to the session's in- and output
func (client *Client) ConnectPipes(reader io.Reader, writer io.Writer) error {
	stdin, err := client.session.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := client.session.StdoutPipe()
	if err != nil {
		return err
	}

	go func() {
		_, _ = io.Copy(writer, stdout)
	}()
	go func() {
		_, _ = io.Copy(stdin, reader)
	}()

	return nil
}

func (client *Client) forward(addr string, localConn net.Conn) error {
	remoteConn, err := client.Dial(addr)
	if err != nil {
		return err
	}

	close := func() {
		localConn.Close()
		remoteConn.Close()
	}
	_ = close

	var once sync.Once
	go func() {
		_, _ = io.Copy(localConn, remoteConn)
		once.Do(close)
	}()
	go func() {
		_, _ = io.Copy(remoteConn, localConn)
		once.Do(close)
	}()

	return nil
}

// StartTunnel creates a new tunnel to the given address, forwarding all
// connections on the given port over the server to the given address. If the
// given address is a SCION address, QUIC is used; else TCP.
func (client *Client) StartTunnel(local netaddr.IPPort, addr string) error {
	var localListener net.Listener
	if strings.Contains(addr, ",") {
		tlsConf := &tls.Config{
			NextProtos: []string{quicutil.SingleStreamProto},
		}
		ql, err := pan.ListenQUIC(context.Background(), local, nil, tlsConf, nil)
		if err != nil {
			return err
		}
		localListener = quicutil.SingleStreamListener{Listener: ql}
	} else {
		// That's right, TCP listen on UDPAddr. XXX replace with netip.AddrPort once available
		tl, err := net.Listen("tcp", local.String())
		if err != nil {
			return err
		}
		localListener = tl
	}

	go func() {
		// FIXME: this will run forever
		defer localListener.Close()
		for {
			localConn, err := localListener.Accept()
			if err != nil {
				log.Debug("Error accepting tunnel listener: ", err)
				continue
			}

			err = client.forward(addr, localConn)
			if err != nil {
				log.Debug("Error forwarding connection: ", err)
				continue
			}
		}
	}()

	return nil
}

// Dial dials the given address over a tunnel to the server. If the given
// address is a SCION address, QUIC is used; else TCP.
func (client *Client) Dial(addr string) (io.ReadWriteCloser, error) {
	if strings.Contains(addr, ",") {
		return tunnelDialSCION(client.client, addr)
	}
	return client.client.Dial("tcp", addr)
}

// CloseSession closes the current session
func (client *Client) CloseSession() {
	client.session.Close()
}

func loadPrivateKey(filePath string) (ssh.AuthMethod, error) {
	absolutePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}

	key, err := ioutil.ReadFile(absolutePath)
	if err != nil {
		return nil, err
	}

	privateKey, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	return ssh.PublicKeys(privateKey), nil
}

func (client *Client) verifyHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
	log.Debug("Checking new host signature host", "remote", remote)

	err := client.knownHostsFileHandler(hostname, remote, key)
	var keyErr *knownhosts.KeyError
	if errors.As(err, &keyErr) {
		if len(keyErr.Want) == 0 {
			// It's an unknown key, prompt user!
			hash := sha256.New()
			hash.Write(key.Marshal())
			if client.promptForForeignKeyConfirmation(hostname, remote, fmt.Sprintf("%x", hash.Sum(nil))) {
				newLine := knownhosts.Line(remote.String(), key)
				err := appendFile(client.knownHostsFilePath, newLine+"\n")
				if err != nil {
					fmt.Printf("Error appending line to known_hosts file %s", err)
				}
				return nil
			}
			return fmt.Errorf("unknown remote host's public key")
		}
		// Host's signature has changed, error!
		return err
	}
	return err
}

func appendFile(fileName, text string) error {
	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(text)
	return err
}
