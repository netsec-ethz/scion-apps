package shttp

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/chaehni/scion-http/quicconn"
	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

type Client struct {
	AddrString string
	Addr       *snet.Addr
}

func (c *Client) Get(serverAddress string) (string, error) {

	// Initialize the SCION/QUIC network connection
	srvAddr, cAddr, err := c.initSCIONConnection(serverAddress)
	if err != nil {
		return "", err
	}

	// Establish QUIC connection to server
	sess, err := squic.DialSCION(nil, cAddr, srvAddr)
	defer sess.Close(nil)
	if err != nil {
		return "", fmt.Errorf("Error dialing SCION: %v", err)
	}

	stream, err := sess.OpenStreamSync()
	defer stream.Close()
	if err != nil {
		return "", fmt.Errorf("Error opening stream: %v", err)
	}

	qc := &quicconn.QuicConn{sess, stream}

	fmt.Fprint(qc, "GET /hello_world.html HTTP/1.1\r\n")
	fmt.Fprint(qc, "Content-Type: text/html\r\n")
	fmt.Fprint(qc, "\r\n")

	buf, _ := ioutil.ReadAll(qc)
	return string(buf), nil

}

func (c *Client) initSCIONConnection(serverAddress string) (*snet.Addr, *snet.Addr, error) {

	log.Println("Initializing SCION connection")

	srvAddr, err := snet.AddrFromString(serverAddress)
	if err != nil {
		return nil, nil, err
	}

	c.Addr, err = snet.AddrFromString(c.AddrString)
	if err != nil {
		return nil, nil, err
	}

	err = snet.Init(c.Addr.IA, utils.GetSciondAddr(c.Addr), utils.GetDispatcherAddr(c.Addr))
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to initialize SCION network:", err)
	}

	log.Println("Initialized SCION network")

	return srvAddr, c.Addr, nil

}
