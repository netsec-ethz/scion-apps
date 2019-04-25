package scionutils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/lucas-clemente/quic-go"

	"github.com/scionproto/scion/go/lib/addr"
	scionlog "github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"

	log "github.com/inconshreveable/log15"

	"github.com/netsec-ethz/scion-apps/ssh/quicconn"
)

var addressPortSplitRegex, _ = regexp.Compile(`(.*,\[.*\]):(\d+)`)

func InitSCIONConnection(tlsKeyFile, tlsCertFile string, useIASCIONDPath bool) error {
	scionlog.SetupLogConsole("debug")

	log.Debug("Initializing SCION connection...")

	localAddress, err := getLocalBindAddress(0)
	if err != nil {
		return err
	}

	localCCAddr, err := snet.AddrFromString(localAddress)
	if err != nil {
		return err
	}

	sciondPath := sciond.GetDefaultSCIONDPath(nil)
	if useIASCIONDPath {
		sciondPath = sciond.GetDefaultSCIONDPath(&localCCAddr.IA)
	}

	err = snet.Init(localCCAddr.IA, sciondPath, "/run/shm/dispatcher/default.sock")
	if err != nil {
		return err
	}

	if tlsKeyFile != "" || tlsCertFile != "" {
		err = squic.Init(tlsKeyFile, tlsCertFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func GetIA() (*addr.IA, error) {
	iaFmt, err := ioutil.ReadFile(filepath.Join(os.Getenv("SC"), "gen/ia"))
	if err != nil {
		return nil, err
	}

	res, err := addr.IAFromFileFmt(string(iaFmt), false)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func SplitHostPort(hostport string) (host, port string, err error) {
	split := addressPortSplitRegex.FindAllStringSubmatch(hostport, -1)
	if len(split) == 1 {
		return split[0][1], split[0][2], nil
	} else {
		// Shouldn't happen
		return "", "", fmt.Errorf("Invalid SCION address provided")
	}
}

func getLocalBindAddress(port uint16) (string, error) {
	defaultIA, err := GetIA()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s,[127.0.0.1]:%v", (*defaultIA).String(), port), nil
}

func DialSCION(remoteAddress string) (*quicconn.QuicConn, error) {
	localAddress, err := getLocalBindAddress(0)
	if err != nil {
		return nil, err
	}

	localCCAddr, err := snet.AddrFromString(localAddress)
	if err != nil {
		return nil, err
	}

	remoteCCAddr, err := snet.AddrFromString(remoteAddress)
	if err != nil {
		return nil, err
	}

	quicConfig := &quic.Config{
		KeepAlive: true,
	}

	sess, err := squic.DialSCION(nil, localCCAddr, remoteCCAddr, quicConfig)
	if err != nil {
		return nil, err
	}

	stream, err := sess.OpenStreamSync()
	if err != nil {
		return nil, err
	}

	return &quicconn.QuicConn{Session: sess, Stream: stream}, nil
}

func ListenSCION(port uint16) (quic.Listener, error) {
	localAddress, err := getLocalBindAddress(port)
	if err != nil {
		return nil, err
	}

	localCCAddr, err := snet.AddrFromString(localAddress)
	if err != nil {
		return nil, err
	}

	listener, err := squic.ListenSCION(nil, localCCAddr, nil)
	if err != nil {
		return nil, err
	}

	return listener, nil
}
