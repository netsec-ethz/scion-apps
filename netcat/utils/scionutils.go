package utils

import (
    "fmt"
    "regexp"
    "log"

    "github.com/lucas-clemente/quic-go"

    scionlog "github.com/scionproto/scion/go/lib/log"
    "github.com/scionproto/scion/go/lib/snet"
    "github.com/scionproto/scion/go/lib/snet/squic"
    "github.com/scionproto/scion/go/lib/sciond"

    "github.com/netsec-ethz/scion-apps/lib/scionutil"
)

var addressPortSplitRegex, _ = regexp.Compile(`(.*,\[.*\]):(\d+)`)

func InitSCION(tlsKeyFile, tlsCertFile string, useIASCIONDPath bool) error {
    scionlog.SetupLogFile("log", "./log", "debug", 1, 30, 100, 0)

    log.Println("Initializing SCION connection...")

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

func SplitHostPort(hostport string) (host, port string, err error){
    split := addressPortSplitRegex.FindAllStringSubmatch(hostport, -1)
    if(len(split)==1){
        return split[0][1], split[0][2], nil
    }else{
        // Shouldn't happen
        return "", "", fmt.Errorf("Invalid SCION address provided")
    }
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

