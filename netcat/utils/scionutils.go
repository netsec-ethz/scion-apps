package utils

import (
    "fmt"
    "regexp"

    "github.com/lucas-clemente/quic-go"

    "github.com/scionproto/scion/go/lib/snet"
    "github.com/scionproto/scion/go/lib/snet/squic"
    "github.com/scionproto/scion/go/lib/sciond"

    "github.com/netsec-ethz/scion-apps/lib/scionutil"
)

var addressPortSplitRegex, _ = regexp.Compile(`(.*,\[.*\]):(\d+)`)

func InitSCION(tlsKeyFile, tlsCertFile string, useIASCIONDPath bool) error {
	localCCAddr, err := scionutil.GetLocalhost()
	if err != nil {
		return err
    }
    
	sciondPath := sciond.GetDefaultSCIONDPath(nil)
	if (useIASCIONDPath) {
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


func DialSCION(remoteAddress string) (*QuicConn, error) {
    localCCAddr, err := scionutil.GetLocalhost()
    if err != nil {
        return nil, err
    }

    remoteCCAddr, err := snet.AddrFromString(remoteAddress)
    if err != nil {
        return nil, err
    }

    quicConfig := &quic.Config {
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
    
    return &QuicConn{Session:sess, Stream:stream}, nil
}

func ListenSCION(port uint16) (quic.Listener, error) {
    localAddress, err := scionutil.GetLocalhostString()
    if err != nil {
        return nil, err
    }

    localCCAddr, err := snet.AddrFromString(fmt.Sprintf("%s:%v", localAddress, port))
    if err != nil {
        return nil, err
    }

    listener, err := squic.ListenSCION(nil, localCCAddr, nil)
    if err != nil {
       return nil, err
    }
    
    return listener, nil
}

