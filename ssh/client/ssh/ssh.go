package ssh

import (
	"crypto/sha256"
	"fmt"
	"github.com/netsec-ethz/scion-apps/ssh/appconf"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	log "github.com/inconshreveable/log15"

	"golang.org/x/crypto/ssh"

	"github.com/netsec-ethz/scion-apps/ssh/client/clientconfig"
	"github.com/netsec-ethz/scion-apps/ssh/client/ssh/knownhosts"
	"github.com/netsec-ethz/scion-apps/ssh/quicconn"
	"github.com/netsec-ethz/scion-apps/ssh/scionutils"
	"github.com/netsec-ethz/scion-apps/ssh/sssh"
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
	appConf *appconf.AppConf
}

// Create creates a new unconnected Client.
func Create(username string, config *clientconfig.ClientConfig, passAuthHandler AuthenticationHandler,
	verifyNewKeyHandler VerifyHostKeyHandler, appConf *appconf.AppConf) (*Client, error) {
	client := &Client{
		config: &ssh.ClientConfig{
			User: username,
		},
		appConf: appConf,
	}

	var authMethods []ssh.AuthMethod

	// Load client private key
	if config.PubkeyAuthentication == "yes" {
		for i := len(config.IdentityFile) - 1; i >= 0; i-- {
			am, err := loadPrivateKey(utils.ParsePath(config.IdentityFile[i]))
			if err != nil {
				log.Debug("Error loading private key at %s, trying next. %s", config.IdentityFile[i], err)
			} else {
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
func (client *Client) Connect(clientAddr string, addr string) error {
	goClient, err := sssh.DialSCIONWithConf(clientAddr, addr, client.config, client.appConf)
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
		io.Copy(writer, stdout)
	}()
	go func() {
		io.Copy(stdin, reader)
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
		io.Copy(localConn, remoteConn)
		once.Do(close)
	}()
	go func() {
		io.Copy(remoteConn, localConn)
		once.Do(close)
	}()

	return nil
}

// StartTunnel creates a new tunnel to the given address, forwarding all connections on the given port over the server to the given address. If the given address is a SCION address, QUIC is used; else TCP.
func (client *Client) StartTunnel(localPort uint16, addr string) error {
	if strings.Contains(addr, ",") {
		localListener, err := scionutils.ListenSCION(localPort)
		if err != nil {
			return err
		}

		go func() {
			defer localListener.Close()
			for {
				sess, err := localListener.Accept()
				if err != nil {
					log.Debug("Error accepting tunnel listener: ", err)
					continue
				}

				stream, err := sess.AcceptStream()
				if err != nil {
					log.Debug("Error accepting tunnel listener session: ", err)
					continue
				}

				err = client.forward(addr, &quicconn.QuicConn{Session: sess, Stream: stream})
				if err != nil {
					log.Debug("Error forwarding connection: ", err)
					continue
				}
			}
		}()
	} else {
		localListener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%v", localPort))
		if err != nil {
			return err
		}

		go func() {
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
	}

	return nil
}

// Dial dials the given address over a tunnel to the server. If the given address is a SCION address, QUIC is used; else TCP.
func (client *Client) Dial(addr string) (net.Conn, error) {
	if strings.Contains(addr, ",") {
		conn, err := client.DialSCION(addr)
		return conn, err
	}
	conn, err := client.DialTCP(addr)
	return conn, err
}

// DialTCP dials the given address using TCP over a tunnel to the server.
func (client *Client) DialTCP(addr string) (net.Conn, error) {
	return client.client.Dial("tcp", addr)
}

// DialSCION dials the given address using QUIC over a tunnel to the server. If the given address is a SCION address, QUIC is used; else TCP.
func (client *Client) DialSCION(addr string) (net.Conn, error) {
	return sssh.TunnelDialSCION(client.client, addr)
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
	log.Debug("Checking new host signature host: %s", remote.String())

	err := client.knownHostsFileHandler(hostname, remote, key)
	if err != nil {
		switch e := err.(type) {
		case *knownhosts.KeyError:
			if len(e.Want) == 0 {
				// It's an unknown key, prompt user!
				hash := sha256.New()
				hash.Write(key.Marshal())
				if client.promptForForeignKeyConfirmation(hostname, remote, fmt.Sprintf("%x", hash.Sum(nil))) {
					newLine := knownhosts.Line([]string{remote.String()}, key)
					err = appendFile(client.knownHostsFilePath, newLine)
					if err != nil {
						fmt.Printf("Error appending line to known_hosts file %s", err)
					}
					return nil
				}
				return fmt.Errorf("unknown remote host's public key")
			}
			// Host's signature has changed, error!
			return err
		default:
			// Unknown error
			return err
		}

	} else {
		return nil
	}
}

func appendFile(fileName, text string) error {
	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = f.WriteString(text); err != nil {
		return err
	}
	f.WriteString("\n")

	return nil
}
