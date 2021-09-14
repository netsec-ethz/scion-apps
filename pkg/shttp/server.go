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

package shttp

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
)

const nextProtoRaw = "raw" // Used for pretend-its-TCP QUIC

// Server wraps a http.Server making it work with SCION
type Server struct {
	*http.Server
}

// ListenAndServe listens for HTTP connections on the SCION address addr and calls Serve
// with handler to handle requests
func ListenAndServe(addr string, handler http.Handler) error {
	s := &Server{
		Server: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}
	return s.ListenAndServe()
}

// ListenAndServe listens for HTTPS connections on the SCION address addr and calls Serve
// with handler to handle requests
func ListenAndServeTLS(addr, certFile, keyFile string, handler http.Handler) error {
	s := &Server{
		Server: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}
	return s.ListenAndServeTLS(certFile, keyFile)
}

func (srv *Server) Serve(l net.Listener) error {
	// Providing a custom listener defeats the purpose of this library.
	panic("not implemented")
}

func (srv *Server) ServeTLS(l net.Listener, certFile, keyFile string) error {
	// Providing a custom listener defeats the purpose of this library.
	panic("not implemented")
}

// ListenAndServe listens for QUIC connections on srv.Addr and
// calls Serve to handle incoming requests
func (srv *Server) ListenAndServe() error {
	listener, err := listen(srv.Addr)
	if err != nil {
		return nil
	}
	defer listener.Close()
	return srv.Server.Serve(listener)
}

func (srv *Server) ListenAndServeTLS(certFile, keyFile string) error {
	listener, err := listen(srv.Addr)
	if err != nil {
		return nil
	}
	defer listener.Close()
	return srv.Server.ServeTLS(listener, certFile, keyFile)
}

func listen(addr string) (net.Listener, error) {
	tlsCfg := &tls.Config{
		NextProtos:   []string{nextProtoRaw},
		Certificates: appquic.GetDummyTLSCerts(),
	}
	laddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	quicListener, err := appquic.Listen(laddr, tlsCfg, nil)
	if err != nil {
		return nil, err
	}
	return singleStreamListener{quicListener}, nil
}

type singleStreamListener struct {
	quic.Listener
}

func (l singleStreamListener) Accept() (net.Conn, error) {
	ctx := context.Background()
	sess, err := l.Listener.Accept(ctx)
	if err != nil {
		return nil, err
	}
	str, err := sess.AcceptStream(ctx)
	return singleStreamSession{sess, str}, err
}

type singleStreamSession struct {
	quic.Session
	quic.Stream
}

func (s singleStreamSession) Close() error {
	s.Stream.Close()
	return s.Session.CloseWithError(0, "")
}
