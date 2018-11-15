package shttp

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"

	"github.com/chaehni/scion-http/utils"
	"github.com/lucas-clemente/quic-go/h2quic"
	"github.com/scionproto/scion/go/lib/snet"
)

// Server wraps a h2quic.Server making it work with SCION
type Server struct {
	Addr string

	s *h2quic.Server
}

var mutex sync.Mutex

// ListenAndServeSCION listens for HTTPS connections on the SCION address addr and calls ServeSCION
// with handler to handle requests
func ListenAndServeSCION(addr, certFile, keyFile string, handler http.Handler) error {

	laddr, err := snet.AddrFromString(addr)
	if err != nil {
		return err
	}

	mutex.Lock()
	if snet.DefNetwork == nil {
		err := snet.Init(laddr.IA, utils.GetSCIOND(), utils.GetDispatcher())
		if err != nil {
			return err
		}
	}
	mutex.Unlock()

	network := snet.DefNetwork
	sconn, err := network.ListenSCION("udp4", laddr)
	if err != nil {
		return err
	}

	return ServeSCION(sconn, handler, certFile, keyFile)
}

// ServeSCION creates a listener on conn and listens for HTTPS connections.
// A new goroutine handles each request using handler
func ServeSCION(conn net.PacketConn, handler http.Handler, certFile, keyFile string) error {

	scionServer := &Server{
		s: &h2quic.Server{
			Server: &http.Server{
				Handler: handler,
			},
		},
	}

	return scionServer.ServeSCION(conn, certFile, keyFile)
}

// ListenAndServeSCION listens for QUIC connections on srv.Addr and
// calls ServeSCION to handle incoming requests
func (srv *Server) ListenAndServeSCION(certFile, keyFile string) error {

	laddr, err := snet.AddrFromString(srv.Addr)
	if err != nil {
		return err
	}

	mutex.Lock()
	if snet.DefNetwork == nil {
		err := snet.Init(laddr.IA, utils.GetSCIOND(), utils.GetDispatcher())
		if err != nil {
			return err
		}
	}
	mutex.Unlock()

	network := snet.DefNetwork
	sconn, err := network.ListenSCION("udp4", laddr)
	if err != nil {
		return err
	}

	return srv.ServeSCION(sconn, certFile, keyFile)

}

// ServeSCION listens on conn and accepts incoming connections
// a goroutine is spawned for every request and handled by srv.srv.handler
func (srv *Server) ServeSCION(conn net.PacketConn, certFile, keyFile string) error {

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

	// set TLS config of underlying http.Server
	srv.s.TLSConfig = config

	return srv.s.Serve(conn)
}

// Close the server immediately, aborting requests and sending CONNECTION_CLOSE frames to connected clients
// Close in combination with ListenAndServeSCION (instead of ServeSCION) may race if it is called before a UDP socket is established
func (srv *Server) Close() error {
	return srv.s.Close()
}
