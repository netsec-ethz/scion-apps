package ssh

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/inconshreveable/log15"

	"golang.org/x/crypto/ssh"

	"github.com/netsec-ethz/scion-apps/ssh/client/ssh/knownhosts"
	"github.com/netsec-ethz/scion-apps/ssh/quicconn"
	"github.com/netsec-ethz/scion-apps/ssh/scionutils"
	"github.com/netsec-ethz/scion-apps/ssh/sssh"
	"github.com/netsec-ethz/scion-apps/ssh/utils"
)

type AuthenticationHandler func() (secret string, err error)
type VerifyHostKeyHandler func(hostname string, remote net.Addr, key string) bool

type SSHClientConfig struct {
	// Host key verification
	VerifyHostKey       bool
	KnownHostKeyFile    string
	VerifyNewKeyHandler VerifyHostKeyHandler

	UsePasswordAuth bool
	PassAuthHandler AuthenticationHandler

	UsePublicKeyAuth bool
	PrivateKeyPaths  []string
}

type SSHClient struct {
	config                          ssh.ClientConfig
	promptForForeignKeyConfirmation VerifyHostKeyHandler
	knownHostsFileHandler           ssh.HostKeyCallback
	knownHostsFilePath              string

	client  *ssh.Client
	session *ssh.Session
}

func Create(username string, config *SSHClientConfig) (*SSHClient, error) {
	client := &SSHClient{
		config: ssh.ClientConfig{
			User: username,
		},
	}

	var authMethods []ssh.AuthMethod

	// Load client private key
	if config.UsePublicKeyAuth {
		for i := len(config.PrivateKeyPaths) - 1; i >= 0; i-- {
			am, err := loadPrivateKey(utils.ParsePath(config.PrivateKeyPaths[i]))
			if err != nil {
				log.Debug("Error loading private key at %s, trying next. %s", config.PrivateKeyPaths[i], err)
			} else {
				authMethods = append(authMethods, am)
			}
		}
	}

	// Use password auth
	if config.UsePasswordAuth {
		log.Debug("Configuring password auth")
		authMethods = append(authMethods, ssh.PasswordCallback(config.PassAuthHandler))
	}

	if config.VerifyHostKey {
		// Create file if doesn't exist
		if _, err := os.Stat(config.KnownHostKeyFile); os.IsNotExist(err) {
			var file, err = os.Create(config.KnownHostKeyFile)
			if err != nil {
				return nil, err
			}
			file.Close()
		}

		client.knownHostsFilePath = config.KnownHostKeyFile
		khh, err := knownhosts.New(config.KnownHostKeyFile)
		if err != nil {
			return nil, err
		}
		client.knownHostsFileHandler = khh
		client.config.HostKeyCallback = client.verifyHostKey
		client.promptForForeignKeyConfirmation = config.VerifyNewKeyHandler
	} else {
		log.Debug("Not verifying host key!")
		client.config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}
	client.config.Auth = authMethods

	return client, nil
}

func (client *SSHClient) Connect(addr string) error {
	goClient, err := sssh.DialSCION(addr, &client.config)
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

func (client *SSHClient) Run(cmd string) error {
	return client.session.Run(cmd)
}

func (client *SSHClient) Start(cmd string) error {
	return client.session.Start(cmd)
}

func (client *SSHClient) Wait() error {
	return client.session.Wait()
}

func (client *SSHClient) ConnectPipes(reader io.Reader, writer io.Writer) error {
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

func (client *SSHClient) forward(addr string, localConn net.Conn) error {
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

func (client *SSHClient) StartTunnel(localPort uint16, addr string) error {
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
					time.Sleep(time.Second)
					continue
				}

				stream, err := sess.AcceptStream()
				if err != nil {
					log.Debug("Error accepting tunnel listener session: ", err)
					time.Sleep(time.Second)
					continue
				}

				err = client.forward(addr, &quicconn.QuicConn{Session: sess, Stream: stream})
				if err != nil {
					log.Debug("Error forwarding connection: ", err)
					time.Sleep(time.Second)
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
					time.Sleep(time.Second)
					continue
				}

				err = client.forward(addr, localConn)
				if err != nil {
					log.Debug("Error forwarding connection: ", err)
					time.Sleep(time.Second)
					continue
				}
			}
		}()
	}

	return nil
}

func (client *SSHClient) Dial(addr string) (net.Conn, error) {
	if strings.Contains(addr, ",") {
		conn, err := client.DialSCION(addr)
		return conn, err
	} else {
		conn, err := client.DialTCP(addr)
		return conn, err
	}
}

func (client *SSHClient) DialTCP(addr string) (net.Conn, error) {
	return client.client.Dial("tcp", addr)
}

func (client *SSHClient) DialSCION(addr string) (net.Conn, error) {
	return sssh.TunnelDialSCION(client.client, addr)
}

func (c *SSHClient) Close() {
	c.session.Close()
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

func (c *SSHClient) verifyHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
	log.Debug("Checking new host signature host: %s", remote.String())

	err := c.knownHostsFileHandler(hostname, remote, key)
	if err != nil {
		switch e := err.(type) {
		case *knownhosts.KeyError:
			if len(e.Want) == 0 {
				// It's an unknown key, prompt user!
				hash := md5.New()
				hash.Write(key.Marshal())
				if c.promptForForeignKeyConfirmation(hostname, remote, fmt.Sprintf("%x", hash.Sum(nil))) {
					newLine := knownhosts.Line([]string{remote.String()}, key)
					err = appendFile(c.knownHostsFilePath, newLine)
					if err != nil {
						fmt.Printf("Error appending line to known_hosts file %s", err)
					}
					return nil
				} else {
					return fmt.Errorf("Unknown remote host's public key!")
				}
			} else {
				// Host's signature has changed, error!
				return err
			}
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
