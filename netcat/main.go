package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	golog "log"
	"os"
	"strconv"
	"sync"

	quic "github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

func printUsage() {
	fmt.Println("netcat [flags] host-address port")
	fmt.Println("netcat [flags] -l port")
	fmt.Println("The host address is specified as ISD-AS,[IP Address]")
	fmt.Println("Example SCION address: 17-ffaa:1:bfd,[127.0.0.1]:42002")
	fmt.Println("Available flags:")
	fmt.Println("  -h: Show help")
	fmt.Println("  -local: Local SCION address (default localhost)")
	fmt.Println("  -b: Send or expect an extra (throw-away) byte before the actual data")
	fmt.Println("  -tlsKey: TLS key path, only needed when listening (default: ./key.pem)")
	fmt.Println("  -tlsCertificate: TLS certificate path, only needed when listening (default: ./certificate.pem)")
}

func main() {
	log.SetupLogConsole("debug")
	log.Debug("Launching netcat")

	var (
		remoteAddressString    string
		port                   uint16
		localAddrString        string

		quicTLSKeyPath         string
		quicTLSCertificatePath string
		
		extraByte              bool
		listen                 bool
	)
	flag.Usage = printUsage
	flag.StringVar(&remoteAddressString, "local", "", "Local address string")
	flag.StringVar(&quicTLSKeyPath, "tlsKey", "./key.pem", "TLS key path")
	flag.StringVar(&quicTLSCertificatePath, "tlsCert", "./certificate.pem", "TLS certificate path")
	flag.BoolVar(&extraByte, "b", false, "Expect extra byte")
	flag.BoolVar(&listen, "l", false, "Listen mode")
	flag.Parse()

	tail := flag.Args()
	if !(len(tail) == 1 && listen) && !(len(tail) == 2 && !listen) {
		printUsage()
		golog.Panicf("Incorrect number of arguments! Arguments: %v", tail)
	}

	remoteAddressString = tail[0]
	port64, err := strconv.ParseUint(tail[len(tail)-1], 10, 16)
	if err != nil {
		printUsage()
		golog.Panicf("Can't parse port string %v: %v", port64, err)
	}
	port = uint16(port64)

	if localAddrString == "" {
		localAddrString, err = scionutil.GetLocalhostString()
		if err != nil {
			golog.Panicf("Error getting localhost: %v", err)
		}
	}

	localAddr, err := snet.AddrFromString(localAddrString)
	if err != nil {
		golog.Panicf("Error parsing local address: %v", err)
	}

	// Initialize SCION library
	err = scionutil.InitSCION(localAddr)
	if err != nil {
		golog.Panicf("Error initializing SCION connection: %v", err)
	}

	var (
		sess   quic.Session
		stream quic.Stream
	)

	if listen {
		err := squic.Init(quicTLSKeyPath, quicTLSCertificatePath)
		if err != nil {
			golog.Panicf("Error initializing squic: %v", err)
		}

		sess, stream = doListen(localAddr, extraByte)
	} else {
		remoteAddr, err := snet.AddrFromString(fmt.Sprintf("%s:%v", remoteAddressString, port))
		if err != nil {
			golog.Panicf("Can't parse remote address %s: %v", remoteAddressString)
		}

		sess, stream = doDial(localAddr, remoteAddr, extraByte)
	}

	close := func() {
		err := stream.Close()
		if err != nil {
			log.Warn("Error closing stream: %v", err)
		}
		err = sess.Close(nil)
		if err != nil {
			log.Warn("Error closing session: %v", err)
		}
	}

	var once sync.Once
	go func() {
		io.Copy(os.Stdout, stream)
		once.Do(close)
	}()
	io.Copy(stream, os.Stdin)
	once.Do(close)
}

func doListen(localAddr *snet.Addr, extraByte bool) (quic.Session, quic.Stream) {
	listener, err := squic.ListenSCION(nil, localAddr, &quic.Config{KeepAlive: true})
	if err != nil {
		golog.Panicf("Can't listen on address %v: %v", localAddr, err)
	}

	sess, err := listener.Accept()
	if err != nil {
		golog.Panicf("Can't accept listener: %v", err)
	}

	stream, err := sess.AcceptStream()
	if err != nil {
		golog.Panicf("Can't accept stream: %v", err)
	}

	log.Debug("Connected!")

	if extraByte {
		(*bufio.NewReader(stream)).ReadByte()

		log.Debug("Received extra byte!")
	}

	return sess, stream
}

func doDial(localAddr, remoteAddr *snet.Addr, extraByte bool) (quic.Session, quic.Stream) {
	sess, err := squic.DialSCION(nil, localAddr, remoteAddr, &quic.Config{KeepAlive: true})
	if err != nil {
		golog.Panicf("Can't dial remote address %v: %v", remoteAddr, err)
	}

	stream, err := sess.OpenStreamSync()
	if err != nil {
		golog.Panicf("Can't open stream: %v", err)
	}

	log.Debug("Connected!")

	if extraByte {
		_, err := stream.Write([]byte{71})
		if err != nil {
			golog.Panicf("Error writing extra byte: %v", err)
		}

		log.Debug("Sent extra byte!")
	}

	return sess, stream
}
