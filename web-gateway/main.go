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

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/handlers"
	"github.com/lucas-clemente/quic-go"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
	"github.com/netsec-ethz/scion-apps/pkg/shttp"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	hosts := kingpin.Arg("hosts", "Hostnames for hosts to proxy").Required().Strings()
	kingpin.Parse()

	// Proxy HTTP:
	mux := http.NewServeMux()
	for _, host := range *hosts {
		u, err := url.Parse(fmt.Sprintf("http://%s/", host))
		if err != nil {
			panic(err)
		}
		mux.Handle(host+"/", httputil.NewSingleHostReverseProxy(u))
	}
	// Fallback: return 502
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "502 bad gateway", http.StatusBadGateway)
	})

	loggedMux := handlers.LoggingHandler(
		os.Stdout,
		mux,
	)
	go func() {
		log.Fatalf("%s", shttp.ListenAndServe(":80", loggedMux))
	}()

	// For HTTPS, forward the entire TLS traffic data
	hostSet := make(map[string]struct{})
	for _, h := range *hosts {
		hostSet[h] = struct{}{}
	}
	log.Fatalf("%s", forwardTLS(hostSet))
}

// forwardTLS listens on 443 and forwards each sessions to the corresponding
// TCP/IP host identified by SNI
func forwardTLS(hosts map[string]struct{}) error {
	listener, err := listen(":443")
	if err != nil {
		return err
	}
	for {
		sess, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}
		go forwardTLSSession(hosts, sess)
	}
}

// forwardTLS forwards traffic for sess to the corresponding TCP/IP host
// identified by SNI.
func forwardTLSSession(hosts map[string]struct{}, sess quic.Session) {
	clientConn, err := sess.AcceptStream(context.Background())
	if err != nil {
		return
	}

	sni := sess.ConnectionState().TLS.ServerName // cheat, use SNI for _underlying_ TLS session in QUIC.
	if _, ok := hosts[sni]; !ok {
		logForwardTLS(sess.RemoteAddr(), sni, 502)
		_ = sess.CloseWithError(502, "bad gateway")
		return
	}
	dstConn, err := net.Dial("tcp", sni+":443")
	if err != nil {
		logForwardTLS(sess.RemoteAddr(), sni, 503)
		_ = sess.CloseWithError(503, "service unavailable")
		return
	}

	logForwardTLS(sess.RemoteAddr(), sni, 200)
	go transfer(dstConn, clientConn)
	transfer(clientConn, dstConn)
}

// logForwardTLS logs TLS forwarding in similar format to HTTP server logs
// Status code is to show something like HTTP codes (arbitrary, this is not HTTP).
func logForwardTLS(client net.Addr, dest string, status int) {
	ts := time.Now().Format("02/Jan/2006:15:04:05 -0700")
	fmt.Printf("%s - - [%s] \"TUNNEL %s\" %d -\n", client, ts, dest, status)
}

func transfer(dst io.WriteCloser, src io.ReadCloser) {
	defer dst.Close()
	defer src.Close()
	buf := make([]byte, 1024)
	var written int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write")
				}
			}
			written += int64(nw)
			if ew != nil || nr != nw {
				break
			}
		}
		if er != nil {
			break
		}
	}
}

func listen(addr string) (quic.Listener, error) {
	laddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	tlsCfg := &tls.Config{
		NextProtos:   []string{"raw"},
		Certificates: appquic.GetDummyTLSCerts(),
	}
	return appquic.Listen(laddr, tlsCfg, nil)
}
