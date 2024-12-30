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

package main

import (
	"context"
	"crypto/tls"
	golog "log"
	"net/netip"
	"os"
	"strconv"

	log "github.com/inconshreveable/log15"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
	"github.com/netsec-ethz/scion-apps/ssh/config"
	"github.com/netsec-ethz/scion-apps/ssh/server/serverconfig"
	"github.com/netsec-ethz/scion-apps/ssh/server/ssh"
	"github.com/netsec-ethz/scion-apps/ssh/utils"
)

const (
	version = "1.0"
)

var (
	// Connection
	options = kingpin.Flag("option", "Set an option").Short('o').Strings()

	// Configuration file
	configurationFile = kingpin.Flag("config-file", "SSH server configuration file").Short('f').Default("/etc/ssh/sshd_config").ExistingFile()
)

func createConfig() *serverconfig.ServerConfig {
	conf := serverconfig.Create()

	updateConfigFromFile(conf, *configurationFile)

	for _, option := range *options {
		err := config.UpdateFromString(conf, option)
		if err != nil {
			log.Debug("Error updating config from --option flag: %v", err)
		}
	}

	// TODO: Set port from listening address
	// setConfIfNot(conf, "Port", *PORT, 0)

	return conf
}

func updateConfigFromFile(conf *serverconfig.ServerConfig, pth string) {
	err := config.UpdateFromFile(conf, utils.ParsePath(pth))
	if err != nil {
		if !os.IsNotExist(err) {
			golog.Panicf("Error updating config from file %s: %v", pth, err)
		}
	}
}

func main() {
	kingpin.Parse()
	log.Debug("Starting SCION SSH server...")

	conf := createConfig()

	sshServer, err := ssh.Create(conf, version)
	if err != nil {
		golog.Panicf("Error creating ssh server: %v", err)
	}

	port, err := strconv.ParseUint(conf.Port, 10, 16)
	if err != nil {
		golog.Panicf("Can't parse port %v: %v", conf.Port, err)
	}
	log.Debug("Currently, ListenAddress.Port is ignored (only value from config taken)")

	local := netip.AddrPortFrom(netip.Addr{}, uint16(port))
	tlsConf := &tls.Config{
		Certificates: quicutil.MustGenerateSelfSignedCert(),
		NextProtos:   []string{quicutil.SingleStreamProto},
	}
	ql, err := pan.ListenQUIC(context.Background(), local, nil, nil, tlsConf, nil)
	if err != nil {
		golog.Panicf("Failed to listen (%v)", err)
	}
	listener := quicutil.SingleStreamListener{Listener: ql}

	log.Debug("Starting to wait for connections")
	for {
		conn, err := listener.Accept()
		if err != nil {
			golog.Fatalf("Failed to accept session: %v", err)
		}
		sshServer.HandleConnection(conn)
	}
}
