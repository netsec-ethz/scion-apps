package hercules

import (
	"fmt"
	"github.com/netsec-ethz/scion-apps/ftp/scion"
	"net"
	"os"
	"os/exec"
	"os/user"
)

var ErrNoFileSystem = fmt.Errorf("driver is not backed by a filesystem")

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

func prepareHerculesArgs(herculesBinary string, herculesConfig *string, localAddress scion.Address) ([]string, error) {
	iface, err := findInterfaceName(localAddress.Addr().Host.IP)
	if err != nil {
		return nil, err
	}

	args := []string{
		herculesBinary,
		"-l", localAddress.String(),
		"-i", iface,
	}

	if herculesConfig != nil {
		args = append(args, "-c", *herculesConfig)
	}
	return args, nil
}

func PrepareHerculesSendCommand(herculesBinary string, herculesConfig *string, localAddress, remoteAddress scion.Address, file string) (*exec.Cmd, error) {
	args, err := prepareHerculesArgs(herculesBinary, herculesConfig, localAddress)
	if err != nil {
		return nil, err
	}

	args = append(args,
		"-t", file,
		"-d", remoteAddress.String(),
	)
	return exec.Command("sudo", args...), nil
}

func PrepareHerculesRecvCommand(herculesBinary string, herculesConfig *string, localAddress scion.Address, file string) (*exec.Cmd, error) {
	args, err := prepareHerculesArgs(herculesBinary, herculesConfig, localAddress)
	if err != nil {
		return nil, err
	}

	args = append(args,
		"-o", file,
		"-timeout", "5",
	)
	return exec.Command("sudo", args...), nil
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
