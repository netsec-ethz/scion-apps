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

package hercules

import (
	"errors"
	"fmt"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/snet"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
)

func findInterfaceName(localAddr net.IP) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("could not retrieve network interfaces: %s", err)
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return "", fmt.Errorf("could not get interface addresses: %s", err)
		}

		if iface.Flags&net.FlagUp == 0 {
			continue // interface not up
		}

		for _, addr := range addrs {
			ip, ok := addr.(*net.IPNet)
			if ok && ip.IP.To4() != nil && ip.IP.To4().Equal(localAddr) {
				return iface.Name, nil
			}
		}
	}

	return "", fmt.Errorf("could not find interface with address %s", localAddr)
}

func prepareHerculesArgs(herculesBinary string, herculesConfig *string, localAddress *net.UDPAddr, offset int64) ([]string, error) {
	iface, err := findInterfaceName(localAddress.IP)
	if err != nil {
		return nil, err
	}

	lAddr := &snet.UDPAddr{
		IA:   appnet.DefNetwork().IA,
		Host: localAddress,
	}

	args := []string{
		herculesBinary,
		"-l", lAddr.String(),
		"-i", iface,
	}

	if herculesConfig != nil {
		args = append(args, "-c", *herculesConfig)
	}
	if offset != -1 {
		args = append(args, "-foffset", strconv.FormatInt(offset, 10))
	}
	return args, nil
}

// PrepareHerculesSendCommand builds an exec.Command to run Hercules in sender mode
// Does not attempt to resolve a configuration file, if herculesConfig is nil
func PrepareHerculesSendCommand(herculesBinary string, herculesConfig *string, localAddress *net.UDPAddr, remoteAddress *snet.UDPAddr, file string, offset int64) (*exec.Cmd, error) {
	args, err := prepareHerculesArgs(herculesBinary, herculesConfig, localAddress, offset)
	if err != nil {
		return nil, err
	}

	args = append(args,
		"-t", file,
		"-d", remoteAddress.String(),
	)
	return exec.Command("sudo", args...), nil
}

// PrepareHerculesRecvCommand builds an exec.Command to run Hercules in receiver mode
// Does not attempt to resolve a configuration file, if herculesConfig is nil
func PrepareHerculesRecvCommand(herculesBinary string, herculesConfig *string, localAddress *net.UDPAddr, file string, offset int64) (*exec.Cmd, error) {
	args, err := prepareHerculesArgs(herculesBinary, herculesConfig, localAddress, offset)
	if err != nil {
		return nil, err
	}

	args = append(args,
		"-o", file,
		"-timeout", "5",
	)
	return exec.Command("sudo", args...), nil
}

func checkIfRegularFile(fileName string) (bool, error) {
	stat, err := os.Stat(fileName)
	if err == nil && stat.Mode().IsRegular() {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ResolveConfig checks for a hercules.toml config file in the following locations:
//  - the current working directory
//  - /etc/scion-ftp/
//  - /etc/
func ResolveConfig() (*string, error) {
	candidates := []string{"hercules.toml", "/etc/hercules.toml", "/etc/scion-ftp/hercules.toml"}
	for _, candidate := range candidates {
		exists, err := checkIfRegularFile(candidate)
		if err != nil {
			return nil, err
		}
		if exists {
			return &candidate, nil
		}
	}
	return nil, nil
}

// AssertFileWriteable checks that the file is writeable with the process owner's user permissions
// If the file does not exist, AssertFileWriteable will attempt to create it
func AssertFileWriteable(path string) (fileCreated bool, err error) {
	fileCreated = false
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		f, err = os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return
		}
		fileCreated = true
	}
	_ = f.Close()
	return
}

func OwnFile(file string) error {
	usr, err := user.Current()
	if err != nil {
		return err
	}

	args := []string{
		"chown",
		fmt.Sprintf("%s:%s", usr.Uid, usr.Gid),
		file,
	}

	cmd := exec.Command("sudo", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
