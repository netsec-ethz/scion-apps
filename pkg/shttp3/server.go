// Copyright 2021 ETH Zurich
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

package shttp3

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/quic-go/quic-go/http3"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

// Server wraps a http3.Server making it work with SCION
type Server struct {
	*http3.Server
}

// ListenAndServe listens on the SCION/UDP address addr and calls the handler
// for HTTP/3 requests on incoming connections. http.DefaultServeMux is used
// when handler is nil.
func ListenAndServe(addr string, certFile, keyFile string, handler http.Handler) error {
	var err error
	certs := make([]tls.Certificate, 1)
	certs[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	s := &Server{
		Server: &http3.Server{
			Server: &http.Server{
				Addr:    addr,
				Handler: handler,
				TLSConfig: &tls.Config{
					Certificates: certs,
				},
			},
		},
	}
	return s.ListenAndServe()
}

// ListenAndServe listens on the UDP address s.Addr and calls s.Handler to
// handle HTTP/3 requests on incoming connections.
func (s *Server) ListenAndServe() error {
	laddr, err := pan.ParseOptionalIPPort(s.Addr)
	if err != nil {
		return err
	}
	sconn, err := pan.ListenUDP(context.Background(), laddr, nil)
	if err != nil {
		return err
	}
	return s.Server.Serve(sconn)
}

func (s *Server) Serve(conn net.PacketConn) error {
	// Providing a custom packet conn defeats the purpose of this library.
	panic("not implemented")
}
