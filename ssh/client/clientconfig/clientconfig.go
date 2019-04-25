package clientconfig

import ()

type ClientConfig struct {
	User                   string   `regex:".*"`
	HostAddress            string   `regex:"(?P<ia>\\d+-[\\d:A-Fa-f]+),\\[(?P<host>[^\\]]+)\\]"`
	Port                   string   `regex:"0*([0-5]?\\d{0,4}|6([0-4]\\d{3}|5([0-4]\\d{2}|5([0-2]\\d|3[0-5]))))"`
	PasswordAuthentication string   `regex:"(yes|no)"`
	PubkeyAuthentication   string   `regex:"(yes|no)"`
	StrictHostKeyChecking  string   `regex:"(yes|no|ask)"`
	IdentityFile           []string `regex:".*"`
	LocalForward           string   `regex:".*"`
	RemoteForward          string   `regex:".*"`
	ProxyCommand           string   `regex:".*"`
	QUICCertificatePath    string   `regex:".*"`
	QUICKeyPath            string   `regex:".*"`
}

func Create() *ClientConfig {
	return &ClientConfig{
		HostAddress: "",
		Port:        "22",
		PasswordAuthentication: "yes",
		PubkeyAuthentication:   "yes",
		StrictHostKeyChecking:  "ask",
		IdentityFile: []string{
			"~/.ssh/id_ed25519",
			"~/.ssh/id_ecdsa",
			"~/.ssh/id_dsa",
			"~/.ssh/id_rsa",
			"~/.ssh/identity",
		},
		LocalForward:        "",
		RemoteForward:       "",
		ProxyCommand:        "",
		QUICCertificatePath: "~/.ssh/quic-conn-certificate.pem",
		QUICKeyPath:         "~/.ssh/quic-conn-key.pem",
	}
}
