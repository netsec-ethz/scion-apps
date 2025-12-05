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
	"fmt"
	golog "log"
	"net"
	"net/netip"
	"os"
	"os/user"
	"strconv"
	"strings"

	log "github.com/inconshreveable/log15"
	"golang.org/x/term"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/ssh/client/clientconfig"
	"github.com/netsec-ethz/scion-apps/ssh/client/ssh"
	"github.com/netsec-ethz/scion-apps/ssh/config"
	"github.com/netsec-ethz/scion-apps/ssh/utils"
)

var (
	// Connection
	serverAddress = kingpin.Arg("host-address", "Server SCION address (without the port)").Required().String()
	runCommand    = kingpin.Arg("command", "Command to run (empty for pty)").Strings()
	port          = kingpin.Flag("port", "The server's port").Default("0").Short('p').Uint16()
	localForward  = kingpin.Flag("local-forward", "Forward remote address connections to listening port. Format: listening_port:remote_address").Short('L').String()
	options       = kingpin.Flag("option", "Set an option").Short('o').Strings()
	configFiles   = kingpin.Flag("config", "Configuration files").Short('c').Default("/etc/ssh/ssh_config", "~/.ssh/config").Strings()
	interactive   = kingpin.Flag("interactive", "Prompt user for interactive path selection").Bool()
	sequence      = kingpin.Flag("sequence", "Sequence of space separated hop predicates to specify path").Default("").String()
	preference    = kingpin.Flag("preference", "Preference sorting order for paths. "+
		"Comma-separated list of available sorting options: "+
		strings.Join(pan.AvailablePreferencePolicies, "|")).Default("").String()
	pathSelector = kingpin.Flag("selector", "Path selection mode").Default("default").Enum(ssh.AvailablePathSelectors...)

	// TODO: additional file paths
	knownHostsFile = kingpin.Flag("known-hosts", "File where known hosts are stored").ExistingFile()
	identityFile   = kingpin.Flag("identity", "Identity (private key) file").Short('i').ExistingFile()

	loginName = kingpin.Flag("login-name", "Username to login with").String()
)

// PromptPassword prompts the user for a password to authenticate with.
func PromptPassword() (secret string, err error) {
	fmt.Printf("Password: ")
	password, _ := term.ReadPassword(0)
	fmt.Println()
	return string(password), nil
}

// PromptAcceptHostKey prompts the user to accept or reject the given host key.
func PromptAcceptHostKey(hostname string, remote net.Addr, publicKey string) bool {
	for {
		fmt.Printf("Key fingerprint SHA256 is: %s do you recognize it? (y/n) ", publicKey)
		var answer string
		_, _ = fmt.Scanln(&answer)
		answer = strings.ToLower(answer)
		if strings.HasPrefix(answer, "y") {
			fmt.Printf("Alright, adding %s to the list of known hosts\n", publicKey)
			return true
		} else if strings.HasPrefix(answer, "n") {
			return false
		} else {
			fmt.Printf("Not a valid answer. Try again")
		}
	}
}

func setConfIfNot(conf *clientconfig.ClientConfig, name string, value, not interface{}) bool {
	res, err := config.SetIfNot(conf, name, value, not)
	if err != nil {
		golog.Panicf("Error setting option %s to %v: %v", name, value, err)
	}
	return res
}

func createConfig() *clientconfig.ClientConfig {
	conf := clientconfig.Create()

	for _, configFile := range *configFiles {
		updateConfigFromFile(conf, configFile)
	}

	for _, option := range *options {
		err := config.UpdateFromString(conf, option)
		if err != nil {
			log.Debug("Error updating config from --option flag: %v", err)
		}
	}

	setConfIfNot(conf, "Port", *port, 0)
	setConfIfNot(conf, "HostAddress", *serverAddress, "")
	setConfIfNot(conf, "IdentityFile", *identityFile, "")
	setConfIfNot(conf, "LocalForward", *localForward, "")
	setConfIfNot(conf, "User", *loginName, "")
	setConfIfNot(conf, "KnownHostsFile", *knownHostsFile, "")

	return conf
}

func updateConfigFromFile(conf *clientconfig.ClientConfig, pth string) {
	err := config.UpdateFromFile(conf, utils.ParsePath(pth))
	if err != nil {
		if !os.IsNotExist(err) {
			golog.Panicf("Error updating config from file %s: %v", pth, err)
		}
	}
}

func main() {
	kingpin.Parse()

	conf := createConfig()

	localUser, err := user.Current()
	if err != nil {
		golog.Panicf("Can't find current user: %s", err)
	}

	verifyNewKeyHandler := PromptAcceptHostKey
	if conf.StrictHostKeyChecking == "yes" {
		verifyNewKeyHandler = func(hostname string, remote net.Addr, key string) bool {
			return false
		}
	}

	remoteUsername := conf.User
	if remoteUsername == "" {
		remoteUsername = localUser.Username
	}
	sshClient, err := ssh.Create(remoteUsername, conf, PromptPassword, verifyNewKeyHandler)
	if err != nil {
		golog.Panicf("Error creating ssh client: %v", err)
	}

	policy, err := pan.PolicyFromCommandline(*sequence, *preference, *interactive)
	if err != nil {
		golog.Fatal(err)
	}

	serverAddress := fmt.Sprintf("%s:%v", conf.HostAddress, conf.Port)

	ctx := context.Background()
	err = sshClient.Connect(ctx, serverAddress, policy, *pathSelector)
	if err != nil {
		golog.Panicf("Error connecting: %v", err)
	}
	defer sshClient.CloseSession()

	if conf.LocalForward != "" {
		localForward := strings.SplitN(conf.LocalForward, ":", 2)

		port, err := strconv.ParseUint(localForward[0], 10, 16)
		if err != nil {
			golog.Panicf("Error parsing forwarding port: %v", err)
		}

		local := netip.AddrPortFrom(netip.Addr{}, uint16(port))
		err = sshClient.StartTunnel(local, localForward[1])
		if err != nil {
			golog.Panicf("Error starting tunnel: %v", err)
		}
	}

	// TODO Don't just join those!
	runCommand := strings.Join((*runCommand)[:], " ")

	if runCommand == "" {
		err = sshClient.Shell()
		if err != nil {
			golog.Panicf("Error starting shell: %v", err)
		}
	} else {
		log.Debug("Running command", "cmd", runCommand)

		err = sshClient.ConnectPipes(os.Stdin, os.Stdout)
		if err != nil {
			golog.Panicf("Error connecting pipes: %v", err)
		}

		err = sshClient.StartSession(runCommand)
		if err != nil {
			golog.Panicf("Error running command: %v", err)
		}

		err = sshClient.WaitSession()
		if err != nil {
			golog.Panicf("Error waiting for command to complete: %v", err)
		}
	}
}
