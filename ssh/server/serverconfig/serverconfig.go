package serverconfig

// ServerConfig is a struct containing configuration for the server.
type ServerConfig struct {
	AuthorizedKeysFile     string `regex:".*"`
	Port                   string `regex:"0*([0-5]?\\d{0,4}|6([0-4]\\d{3}|5([0-4]\\d{2}|5([0-2]\\d|3[0-5]))))"`
	PasswordAuthentication string `regex:"(yes|no)"`
	PubkeyAuthentication   string `regex:"(yes|no)"`
	HostKey                string `regex:".*"`
	MaxAuthTries           string `regex:"[1-9]\\d*"`
}

// Create creates a new ServerConfig with the default values.
func Create() *ServerConfig {
	return &ServerConfig{
		AuthorizedKeysFile:     ".ssh/authorized_keys",
		Port:                   "22",
		PasswordAuthentication: "yes",
		PubkeyAuthentication:   "yes",
		HostKey:                "/etc/ssh/ssh_host_key",
	}
}
