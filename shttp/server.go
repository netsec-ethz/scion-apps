package shttp

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/chaehni/scion-http/utils"
	"github.com/lucas-clemente/quic-go/h2quic"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

type Server struct { // TODO: make public?
	AddrString  string
	Addr        *snet.Addr
	TLSCertFile string
	TLSKeyFile  string
	Mux         *http.ServeMux

	srv *h2quic.Server
}

/* Start of public methods */

func Serve(l net.Listener, handler http.Handler) error {
	return http.Serve(l, handler)
}

func ServeTLS(l net.Listener, handler http.Handler, certFile, keyFile string) error {
	return http.ServeTLS(l, handler, certFile, keyFile)
}

func ListenAndServeSCION(addr, certFile, keyFile string, handler http.Handler) error {

	// create TLS config

	var err error
	certs := make([]tls.Certificate, 1)
	certs[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	config := &tls.Config{
		Certificates: certs,
	}

	// start wrapping the servers

	httpServer := &http.Server{
		Handler:   handler,
		TLSConfig: config,
	}

	quicServer := &h2quic.Server{
		Server: httpServer,
	}

	scionServer := &Server{
		AddrString:  addr,
		TLSCertFile: certFile,
		TLSKeyFile:  keyFile,
		srv:         quicServer,
	}

	// Initialize the SCION/QUIC network connection

	//TODO: check if snet.DefNetwork is already initialized
	if _, err := scionServer.initSCIONConnection(); err != nil {
		return err
	}

	laddr, err := snet.AddrFromString(addr)
	if err != nil {
		return err
	}

	network := snet.DefNetwork
	conn, err := network.ListenSCION("udp4", laddr)
	if err != nil {
		return err
	}

	return scionServer.srv.Serve(conn)
}

func (srv *Server) initSCIONConnection() (*snet.Addr, error) {

	log.Println("Initializing SCION connection")

	var err error
	srv.Addr, err = snet.AddrFromString(srv.AddrString)
	if err != nil {
		return nil, err
	}

	err = snet.Init(srv.Addr.IA, utils.GetSCIOND(), utils.GetDispatcher())
	if err != nil {
		return nil, fmt.Errorf("Unable to initialize SCION network: %v", err)
	}

	log.Println("Initialized SCION network")

	err = squic.Init(srv.TLSKeyFile, srv.TLSCertFile)
	if err != nil {
		return nil, fmt.Errorf("Unable to initialize QUIC network: %v", err)
	}

	log.Println("Initialized SCION/QUIC network")

	return srv.Addr, nil

}
