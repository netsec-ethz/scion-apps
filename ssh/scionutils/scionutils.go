package scionutils

import (
	"crypto/tls"
	"fmt"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
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
	return dialSCION(localAddress, remoteAddress, squic.DialSCION)
}

// DialSCIONWithConf performs the same funcionality as DialSCION, but with a SCION connection that is aware of
// user-defined configurations specified in scionutil.PathAppConf
func DialSCIONWithConf(localAddress string, remoteAddress string, appConf *PathAppConf) (*quicconn.QuicConn, error) {
	return dialSCION(localAddress, remoteAddress, squicDialWithConf(appConf))
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

type squicDial func(network *snet.SCIONNetwork, laddr, raddr *snet.Addr,
	quicConfig *quic.Config) (quic.Session, error)

func dialSCION(localAddress string, remoteAddress string, dialer squicDial) (*quicconn.QuicConn, error) {
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

	sess, err := dialer(nil, localCCAddr, remoteCCAddr, quicConfig)
	if err != nil {
		return nil, err
	}

	stream, err := sess.OpenStreamSync()
	if err != nil {
		return nil, err
	}

	return &quicconn.QuicConn{Session: sess, Stream: stream}, nil
}

//partially applied function to wrap the SICONConn passed to quic.Dial in a ConnWrapper object
func squicDialWithConf(conf *PathAppConf) squicDial {

	return func(network *snet.SCIONNetwork, laddr, raddr *snet.Addr, quicConfig *quic.Config) (session quic.Session, e error) {
		sconn, err := snet.DefNetwork.ListenSCIONWithBindSVC("udp4", laddr, nil, addr.SvcNone, 0)
		if err != nil {
			return nil, common.NewBasicError("ConnWrapper: error listening SCION", err)
		}
		wrappedConn, err := conf.ConnWrapperFromConfig(sconn) // policyConn takes a SCIONConn and an PathAppConf
		return quic.Dial(wrappedConn, raddr, "host:0", &tls.Config{InsecureSkipVerify: true}, quicConfig)
	}
}
