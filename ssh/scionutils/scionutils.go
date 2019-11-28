package scionutils

import (
	"crypto/tls"
	"fmt"
	"github.com/netsec-ethz/scion-apps/ssh/conn_wrapper"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/netsec-ethz/scion-apps/ssh/appconf"
	"regexp"

	"github.com/lucas-clemente/quic-go"

	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"

	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/netsec-ethz/scion-apps/ssh/quicconn"
)

var addressPortSplitRegex, _ = regexp.Compile(`(.*,\[.*\]):(\d+)`)

// SplitHostPort splits a host:port string into host and port variables
func SplitHostPort(hostport string) (host, port string, err error) {
	split := addressPortSplitRegex.FindAllStringSubmatch(hostport, -1)
	if len(split) == 1 {
		return split[0][1], split[0][2], nil
	}
	// Shouldn't happen
	return "", "", fmt.Errorf("Invalid SCION address provided")
}

// DialSCION dials a SCION host and opens a new QUIC stream
func DialSCION(localAddress string, remoteAddress string) (*quicconn.QuicConn, error) {
	if localAddress == "" {
		localhost, err := scionutil.GetLocalhostString()
		if err != nil {
			return nil, err
		}

		localAddress = fmt.Sprintf("%v:%v", localhost, 0)
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
//
func DialSCIONWithConf(localAddress string, remoteAddress string, appConf *appconf.AppConf) (*quicconn.QuicConn, error) {
	if localAddress == "" {
		localhost, err := scionutil.GetLocalhostString()
		if err != nil {
			return nil, err
		}

		localAddress = fmt.Sprintf("%v:%v", localhost, 0)
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

	sess, err := DialSCIONWithBindSVCWithConf(localCCAddr, remoteCCAddr, nil, addr.SvcNone, quicConfig, appConf)
	if err != nil {
		return nil, err
	}

	stream, err := sess.OpenStreamSync()
	if err != nil {
		return nil, err
	}

	return &quicconn.QuicConn{Session: sess, Stream: stream}, nil
}
// ListenSCION listens on the given port with the QUIC protocol, and returns a listener
func ListenSCION(port uint16) (quic.Listener, error) {
	localhost, err := scionutil.GetLocalhostString()
	if err != nil {
		return nil, err
	}

	localAddress := fmt.Sprintf("%v:%v", localhost, port)

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

func DialSCIONWithBindSVCWithConf(laddr, raddr, baddr *snet.Addr,
	svc addr.HostSVC, quicConfig *quic.Config, conf *appconf.AppConf) (quic.Session, error) {

	sconn, err := snet.DefNetwork.ListenSCIONWithBindSVC("udp4", laddr, baddr, svc, 0)
	wrappedConn := conn_wrapper.NewConnWrapper(sconn, conf) // ConnWrapper takes a SCIONConn and an AppConf
	if err != nil {
		return nil, err
	}
	// Use dummy hostname, as it's used for SNI, and we're not doing cert verification.
	return quic.Dial(wrappedConn, raddr, "host:0", &tls.Config{InsecureSkipVerify:true}, quicConfig)
}




