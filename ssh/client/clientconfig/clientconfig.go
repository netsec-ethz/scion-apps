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

package clientconfig

// ClientConfig is a struct containing configuration for the client.
type ClientConfig struct {
	User                   string   `regex:".*"`
	HostAddress            string   `regex:"([-.\\da-zA-Z]+)|(\\d+-[\\d:A-Fa-f]+,\\[[^\\]]+\\])"`
	Port                   string   `regex:"0*([0-5]?\\d{0,4}|6([0-4]\\d{3}|5([0-4]\\d{2}|5([0-2]\\d|3[0-5]))))"`
	PasswordAuthentication string   `regex:"(yes|no)"`
	PubkeyAuthentication   string   `regex:"(yes|no)"`
	StrictHostKeyChecking  string   `regex:"(yes|no|ask)"`
	IdentityFile           []string `regex:".*"`
	LocalForward           string   `regex:".*"`
	RemoteForward          string   `regex:".*"`
	UserKnownHostsFile     string   `regex:".*"`
	ProxyCommand           string   `regex:".*"`
}

// Create creates a new ClientConfig with the default values.
func Create() *ClientConfig {
	return &ClientConfig{
		User:                   "",
		HostAddress:            "",
		Port:                   "22",
		PasswordAuthentication: "yes",
		PubkeyAuthentication:   "yes",
		StrictHostKeyChecking:  "ask",
		UserKnownHostsFile:     "~/.ssh/known_hosts",
		IdentityFile: []string{
			"~/.ssh/id_ed25519",
			"~/.ssh/id_ecdsa",
			"~/.ssh/id_dsa",
			"~/.ssh/id_rsa",
			"~/.ssh/identity",
		},
		LocalForward:  "",
		RemoteForward: "",
		ProxyCommand:  "",
	}
}
