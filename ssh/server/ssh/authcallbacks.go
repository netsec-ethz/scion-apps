package ssh

import (
	"fmt"
	"io/ioutil"

	"golang.org/x/crypto/ssh"

	"github.com/msteinert/pam"
)

// PasswordAuth authenticates the client using password authentication.
func (s *Server) PasswordAuth(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
	t, err := pam.StartFunc("", c.User(), func(s pam.Style, msg string) (string, error) {
		switch s {
		case pam.PromptEchoOff:
			return string(pass), nil
		}
		return "", fmt.Errorf("unsupported message style")
	})
	if err != nil {
		return nil, err
	}
	err = t.Authenticate(0)
	if err != nil {
		return nil, fmt.Errorf("authenticate: %s", err.Error())
	}

	return &ssh.Permissions{
		CriticalOptions: map[string]string{
			"user": c.User(),
		},
	}, nil
}

func loadAuthorizedKeys(file string) (map[string]bool, error) {
	authKeys := make(map[string]bool)

	authorizedKeysBytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	for len(authorizedKeysBytes) > 0 {
		pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			return nil, err
		}

		authKeys[string(pubKey.Marshal())] = true
		authorizedKeysBytes = rest
	}

	return authKeys, nil
}

// PublicKeyAuth authenticates the client using a public key.
func (s *Server) PublicKeyAuth(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
	authKeys, err := loadAuthorizedKeys(s.authorizedKeysFile)
	if err != nil {
		return nil, fmt.Errorf("failed loading authorized files: %v", err)
	}

	if authKeys[string(pubKey.Marshal())] {
		return &ssh.Permissions{
			CriticalOptions: map[string]string{
				"user": c.User(),
			},
			Extensions: map[string]string{
				// Record the public key used for authentication
				"pubkey-fp": ssh.FingerprintSHA256(pubKey),
			},
		}, nil
	}

	return nil, fmt.Errorf("unknown public key for %q", c.User())
}
