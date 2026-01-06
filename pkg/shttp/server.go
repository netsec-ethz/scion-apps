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

	"github.com/quic-go/quic-go"
	"github.com/scionproto/scion/pkg/snet"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
)

// Server wraps a http.Server making it work with SCION
type Server struct {
	*http.Server
	ASContext pan.ASContext
}

// ListenAndServe listens for HTTP connections on the SCION address addr and calls Serve
// with handler to handle requests.
func ListenAndServe(asCtx pan.ASContext, addr string, handler http.Handler) error {
	s := &Server{
		Server: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
		ASContext: asCtx,
	}
	return s.ListenAndServe()
}

// ListenAndServeTLS listens for HTTPS connections on the SCION address addr and calls Serve
// with handler to handle requests.
func ListenAndServeTLS(asCtx pan.ASContext, addr, certFile, keyFile string, handler http.Handler) error {
	s := &Server{
		Server: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
		ASContext: asCtx,
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
// calls Serve to handle incoming requests.
// Note: The Server must have its ASContext field set before calling this method.
func (srv *Server) ListenAndServe() error {
	listener, err := srv.listen()
	if err != nil {
		return err
	}
	defer listener.Close()
	return srv.Server.Serve(listener)
}

// ListenAndServeTLS listens for QUIC connections on srv.Addr and
// calls ServeTLS to handle incoming requests.
// Note: The Server must have its ASContext field set before calling this method.
func (srv *Server) ListenAndServeTLS(certFile, keyFile string) error {
	listener, err := srv.listen()
	if err != nil {
		return err
	}
	defer listener.Close()
	return srv.Server.ServeTLS(listener, certFile, keyFile)
}

func (srv *Server) listen() (net.Listener, error) {
	tlsCfg := &tls.Config{
		NextProtos:   []string{quicutil.SingleStreamProto},
		Certificates: quicutil.MustGenerateSelfSignedCert(),
	}
	laddr, err := pan.ParseOptionalIPPort(srv.Addr)
	if err != nil {
		return nil, err
	}

	localAddr := &snet.UDPAddr{
		IA:   srv.ASContext.IA(),
		Host: net.UDPAddrFromAddrPort(laddr),
	}

	quicListener, err := pan.ListenQUIC(context.Background(), srv.ASContext, localAddr, tlsCfg, &quic.Config{}, nil)
	if err != nil {
		return nil, err
	}
	return &quicutil.SingleStreamListener{QUICListener: quicListener}, nil
}
