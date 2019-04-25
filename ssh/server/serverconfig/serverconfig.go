package serverconfig

import ()

type ServerConfig struct {
	AuthorizedKeysFile     string `regex:".*"`
	Port                   string `regex:"0*([0-5]?\\d{0,4}|6([0-4]\\d{3}|5([0-4]\\d{2}|5([0-2]\\d|3[0-5]))))"`
	PasswordAuthentication string `regex:"(yes|no)"`
	PubkeyAuthentication   string `regex:"(yes|no)"`
	HostKey                string `regex:".*"`
	QUICCertificatePath    string `regex:".*"`
	QUICKeyPath            string `regex:".*"`
	MaxAuthTries           string `regex:"[1-9]\\d*"`
}

func Create() *ServerConfig {
	return &ServerConfig{
		AuthorizedKeysFile: ".ssh/authorized_keys",
		Port:               "22",
		PasswordAuthentication: "yes",
		PubkeyAuthentication:   "yes",
		HostKey:                "/etc/ssh/ssh_host_key",
		QUICCertificatePath:    "/etc/ssh/quic-conn-certificate.pem",
		QUICKeyPath:            "/etc/ssh/quic-conn-key.pem",
	}
}
