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

import (
	"fmt"
	"strings"
	"testing"

	"github.com/netsec-ethz/scion-apps/ssh/config"
	. "github.com/smartystreets/goconvey/convey"
)

func TestDefaultConfig(t *testing.T) {
	Convey("Given an example SSH config file", t, func() {
		configString := `
			Host *
			ForwardAgent no
			ForwardX11 no
			ForwardX11Trusted yes
			RhostsRSAAuthentication no
			RSAAuthentication yes
			PasswordAuthentication no
			HostbasedAuthentication yes
			GSSAPIAuthentication no
			GSSAPIDelegateCredentials no
			GSSAPIKeyExchange no
			GSSAPITrustDNS no
			BatchMode no
			CheckHostIP yes
			AddressFamily any
			ConnectTimeout 0
			StrictHostKeyChecking no
			IdentityFile ~/.ssh/identity
			IdentityFile ~/.ssh/id_rsa
			IdentityFile ~/.ssh/id_dsa
			IdentityFile ~/.ssh/id_ecdsa
			IdentityFile ~/.ssh/id_ed25519
			Port 65535
			Protocol 2
			Cipher 3des
			Ciphers aes128-ctr,aes192-ctr,aes256-ctr$
			MACs hmac-md5,hmac-sha1,umac-64@openssh.$
			EscapeChar ~
			Tunnel no
			TunnelDevice any:any
			PermitLocalCommand no
			VisualHostKey no
			ProxyCommand ssh -q -W %h:%p gateway.exa$
			RekeyLimit 1G 1h
			SendEnv LANG LC_*
			HashKnownHosts yes
			GSSAPIAuthentication yes
			GSSAPIDelegateCredentials no
		`

		Convey("The new values are read correctly", func() {
			conf := &ClientConfig{}
			config.UpdateFromReader(conf, strings.NewReader(configString))
			So(conf.HostAddress, ShouldEqual, "")
			So(conf.PasswordAuthentication, ShouldEqual, "no")
			So(conf.StrictHostKeyChecking, ShouldEqual, "no")
			So(conf.Port, ShouldEqual, "65535")
			So(conf.IdentityFile[len(conf.IdentityFile)-1], ShouldEqual, "~/.ssh/identity")
			So(conf.IdentityFile[len(conf.IdentityFile)-2], ShouldEqual, "~/.ssh/id_rsa")
			So(conf.IdentityFile[len(conf.IdentityFile)-3], ShouldEqual, "~/.ssh/id_dsa")
			So(conf.IdentityFile[len(conf.IdentityFile)-4], ShouldEqual, "~/.ssh/id_ecdsa")
			So(conf.IdentityFile[len(conf.IdentityFile)-5], ShouldEqual, "~/.ssh/id_ed25519")
		})

	})
}

func TestPortRegex(t *testing.T) {
	Convey("Given a default config file", t, func() {
		conf := &ClientConfig{}

		Convey("Valid port numbers are accepted", func() {
			nums := []int{1, 2, 3, 10, 100, 1000, 10000, 45, 652, 3486, 43621, 6554, 66, 65535}
			for _, i := range nums {
				err := config.Set(conf, "Port", i)
				So(err, ShouldEqual, nil)
				So(conf.Port, ShouldEqual, fmt.Sprintf("%v", i))
			}
		})

		Convey("Invalid port numbers are not accepted", func() {
			nums := []int{-1, 2, 3, 351000, 10064300, 455635, 65345632, 34845636, 6436554, 65536, 70000}
			for _, i := range nums {
				if i >= 0 && i <= 65535 {
					continue
				}
				initialPort := conf.Port
				err := config.Set(conf, "Port", i)
				So(err, ShouldNotEqual, nil)
				So(conf.Port, ShouldEqual, fmt.Sprintf("%v", initialPort))
			}
		})

	})
}
