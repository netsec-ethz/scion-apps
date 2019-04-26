package main

import (
	"fmt"
	golog "log"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"gopkg.in/alecthomas/kingpin.v2"

	scionlog "github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"

	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/netsec-ethz/scion-apps/ssh/client/clientconfig"
	"github.com/netsec-ethz/scion-apps/ssh/client/ssh"
	"github.com/netsec-ethz/scion-apps/ssh/config"
	"github.com/netsec-ethz/scion-apps/ssh/utils"

	log "github.com/inconshreveable/log15"
)

var (
	// Connection
	SERVER_ADDRESS = kingpin.Arg("host-address", "Server SCION address (without the port)").Required().String()
	RUN_COMMAND    = kingpin.Arg("command", "Command to run (empty for pty)").Strings()
	PORT           = kingpin.Flag("port", "The server's port").Default("0").Short('p').Uint16()
	LOCAL_FORWARD  = kingpin.Flag("local-forward", "Forward remote address connections to listening port. Format: listening_port:remote_address").Short('L').String()
	OPTIONS        = kingpin.Flag("option", "Set an option").Short('o').Strings()
	VERBOSE        = kingpin.Flag("verbose", "Be verbose").Short('v').Default("false").Bool()
	CONFIG_FILES   = kingpin.Flag("config", "Configuration files").Short('c').Default("/etc/ssh/ssh_config", "~/.ssh/config").Strings()
	X_DEAD         = kingpin.Flag("x-dead", "Placeholder for SCP support").Short('x').Default("false").Bool()

	// TODO: additional file paths
	KNOWN_HOSTS_FILE = kingpin.Flag("known-hosts", "File where known hosts are stored").ExistingFile()
	IDENTITY_FILE    = kingpin.Flag("identity", "Identity (private key) file").Short('i').ExistingFile()

	USER = kingpin.Flag("login-name", "Username to login with").String()
)

var clientCCAddr *snet.Addr

func PromptPassword() (secret string, err error) {
	fmt.Printf("Password: ")
	password, _ := terminal.ReadPassword(0)
	fmt.Println()
	return string(password), nil
}

func PromptAcceptHostKey(hostname string, remote net.Addr, publicKey string) bool {
	for {
		fmt.Printf("Key fingerprint MD5 is: %s do you recognize it? (y/n) ", publicKey)
		var answer string
		fmt.Scanln(&answer)
		answer = strings.ToLower(answer)
		if strings.HasPrefix(answer, "y") {
			fmt.Printf("Alright, adding %s to the list of known hosts", publicKey)
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

	for _, configFile := range *CONFIG_FILES {
		updateConfigFromFile(conf, configFile)
	}

	for _, option := range *OPTIONS {
		err := config.UpdateFromString(conf, option)
		if err != nil {
			log.Debug("Error updating config from --option flag: %v", err)
		}
	}

	setConfIfNot(conf, "Port", *PORT, 0)
	setConfIfNot(conf, "HostAddress", *SERVER_ADDRESS, "")
	setConfIfNot(conf, "IdentityFile", *IDENTITY_FILE, "")
	setConfIfNot(conf, "User", *USER, "")

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
	scionlog.SetupLogConsole("debug")

	conf := createConfig()

	localUser, err := user.Current()
	if err != nil {
		golog.Panicf("Can't find current user: %s", err)
	}

	knownHostsFile := *KNOWN_HOSTS_FILE
	if knownHostsFile == "" {
		knownHostsFile = "~/.ssh/known_hosts"
	}
	knownHostsFile = utils.ParsePath(knownHostsFile)

	localhost, err := scionutil.GetLocalhost()
	if err != nil {
		golog.Panicf("Can't get localhost: %v", err)
	}

	err = scionutil.InitSCION(localhost)
	if err != nil {
		golog.Panicf("Error initializing SCION: %v", err)
	}

	err = squic.Init(utils.ParsePath(conf.QUICKeyPath), utils.ParsePath(conf.QUICCertificatePath))
	if err != nil {
		golog.Panicf("Error initializing SQUIC: %v", err)
	}

	verifyNewKeyHandler := PromptAcceptHostKey
	if conf.StrictHostKeyChecking == "yes" {
		verifyNewKeyHandler = func(hostname string, remote net.Addr, key string) bool {
			return false
		}
	}

	// Create SSH client
	sshConfig := &ssh.SSHClientConfig{
		VerifyHostKey:       conf.StrictHostKeyChecking != "no",
		VerifyNewKeyHandler: verifyNewKeyHandler,
		KnownHostKeyFile:    knownHostsFile,

		UsePasswordAuth: conf.PasswordAuthentication == "yes",
		PassAuthHandler: PromptPassword,

		UsePublicKeyAuth: conf.PubkeyAuthentication == "yes",
		PrivateKeyPaths:  conf.IdentityFile,
	}

	remoteUsername := conf.User
	if remoteUsername == "" {
		remoteUsername = localUser.Username
	}

	sshClient, err := ssh.Create(remoteUsername, sshConfig)
	if err != nil {
		golog.Panicf("Error creating ssh client: %v", err)
	}

	serverAddress := fmt.Sprintf("%s:%v", conf.HostAddress, conf.Port)

	err = sshClient.Connect(serverAddress)
	if err != nil {
		golog.Panicf("Error connecting: %v", err)
	}
	defer sshClient.Close()

	if conf.LocalForward != "" {
		localForward := strings.SplitN(conf.LocalForward, ":", 2)

		port, err := strconv.ParseUint(localForward[0], 10, 16)
		if err != nil {
			golog.Panicf("Error parsing forwarding port: %v", err)
		}

		err = sshClient.StartTunnel(uint16(port), localForward[1])
		if err != nil {
			golog.Panicf("Error starting tunnel: %v", err)
		}
	}

	// TODO Don't just join those!
	runCommand := strings.Join((*RUN_COMMAND)[:], " ")

	if runCommand == "" {
		err = sshClient.Shell()
		if err != nil {
			golog.Panicf("Error starting shell: %v", err)
		}
	} else {
		log.Debug("Running command: %s", runCommand)

		err = sshClient.ConnectPipes(os.Stdin, os.Stdout)
		if err != nil {
			golog.Panicf("Error connecting pipes: %v", err)
		}

		err = sshClient.Start(runCommand)
		if err != nil {
			golog.Panicf("Error running command: %v", err)
		}

		err = sshClient.Wait()
		if err != nil {
			golog.Panicf("Error waiting for command to complete: %v", err)
		}
	}
}
