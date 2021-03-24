// Copyright 2018 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shttp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/lucas-clemente/quic-go/http3"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

// Server wraps a http3.Server making it work with SCION
type Server struct {
	*http3.Server
}

// ListenAndServe listens for HTTPS connections on the SCION address addr and calls Serve
// with handler to handle requests
func ListenAndServe(addr string, handler http.Handler, tlsConfig *tls.Config) error {

	scionServer := &Server{
		Server: &http3.Server{
			Server: &http.Server{
				Addr:    addr,
				Handler: handler,
			},
		},
	}
	scionServer.TLSConfig = tlsConfig
	return scionServer.ListenAndServe()
}

// Serve creates a listener on conn and listens for HTTPS connections.
// A new goroutine handles each request using handler
func Serve(conn net.PacketConn, handler http.Handler) error {

	scionServer := &Server{
		Server: &http3.Server{
			Server: &http.Server{
				Handler: handler,
			},
		},
	}

	return scionServer.Serve(conn)
}

// ListenAndServe listens for QUIC connections on srv.Addr and
// calls Serve to handle incoming requests
func (srv *Server) ListenAndServe() error {

	laddr, err := net.ResolveUDPAddr("udp", srv.Addr)
	if err != nil {
		return err
	}
	sconn, err := pan.ListenUDP(context.Background(), laddr, nil)
	if err != nil {
		return err
	}
	defer sconn.Close()
	fmt.Println(sconn.LocalAddr())
	return srv.Serve(sconn)
}

// Serve listens on conn and accepts incoming connections
// a goroutine is spawned for every request and handled by srv.srv.handler
func (srv *Server) Serve(conn net.PacketConn) error {

	// set dummy TLS config if not set:
	if srv.TLSConfig == nil {
		srv.TLSConfig = &tls.Config{}
	}
	if len(srv.TLSConfig.Certificates) == 0 {
		srv.TLSConfig.Certificates = appquic.GetDummyTLSCerts()
	}

	return srv.Server.Serve(conn)
}

// Close the server immediately, aborting requests and sending CONNECTION_CLOSE frames to connected clients
// Close in combination with ListenAndServe (instead of Serve) may race if it is called before a UDP socket is established
func (srv *Server) Close() error {
	return srv.Server.Close()
}
