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

//go:build go1.16
// +build go1.16

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/gorilla/handlers"
	"gopkg.in/alecthomas/kingpin.v2"
	"inet.af/netaddr"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
	"github.com/netsec-ethz/scion-apps/pkg/shttp"
)

var (
	mungedScionAddr = regexp.MustCompile(`^(\d+)-([_\dA-Fa-f]+)-(.*)$`)
)

const (
	mungedScionAddrIAIndex   = 1
	mungedScionAddrASIndex   = 2
	mungedScionAddrHostIndex = 3
)

//go:embed skip.pac
var skipPAC string
var skipPACtemplate = template.Must(template.New("skip.pac").Parse(skipPAC))

type skipPACTemplateParams struct {
	ProxyAddress string
	SCIONHosts   []string
}

func main() {
	var bindAddress *net.TCPAddr
	kingpin.Flag("bind", "Address to bind on").Default("localhost:8888").TCPVar(&bindAddress)
	kingpin.Parse()

	proxy := &proxyHandler{
		transport: shttp.DefaultTransport,
	}
	mux := http.NewServeMux()
	mux.Handle("localhost/skip.pac", http.HandlerFunc(handleWPAD))
	if bindAddress.IP != nil {
		mux.Handle(bindAddress.IP.String()+"/skip.pac", http.HandlerFunc(handleWPAD))
	}
	mux.Handle("/scion-host", http.HandlerFunc(handleHostListRequest))
	mux.Handle("/setISDPolicy", http.HandlerFunc(proxy.setISDPolicy))
	mux.Handle("/", proxy) // everything else

	handler := interceptConnect(http.HandlerFunc(proxy.handleTunneling), mux)

	server := &http.Server{
		Addr:    bindAddress.String(),
		Handler: handlers.LoggingHandler(os.Stdout, handler),
	}
	log.Fatal(server.ListenAndServe())
}

func handleWPAD(w http.ResponseWriter, req *http.Request) {
	buf := &bytes.Buffer{}
	err := skipPACtemplate.Execute(buf,
		skipPACTemplateParams{
			ProxyAddress: req.Host,
			SCIONHosts:   loadHosts(),
		},
	)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "error executing template", 500)
		return
	}
	w.Header().Set("content-type", "application/x-ns-proxy-autoconfig")
	_, _ = w.Write(buf.Bytes())
}

func handleHostListRequest(w http.ResponseWriter, req *http.Request) {
	buf := &bytes.Buffer{}
	scionHost := loadHosts()
	if len(scionHost) > 0 {
		buf.WriteString(scionHost[0])
	}
	for _, s := range scionHost[1:] {
		buf.WriteString("\n" + s)
	}
	w.Write(buf.Bytes())
}

type proxyHandler struct {
	transport http.RoundTripper
	policy    pan.Policy
}

func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	host := demunge(req.Host)
	req.Host = host
	req.URL.Host = host

	resp, err := h.transport.RoundTrip(req)
	if err != nil {
		log.Default().Println("error round trip", err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func (h *proxyHandler) setISDPolicy(w http.ResponseWriter, req *http.Request) {
	var aclInput []string
	if req.Method == http.MethodPut {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			fmt.Println(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		aclStringList := strings.Split(string(body), ",")
		aclStringList = aclStringList[:len(aclStringList)-1]
		// Whitelist ases
		for _, entry := range aclStringList {
			aclInput = append(aclInput, "+ "+entry)
		}
		aclInput = append(aclInput, "-")

		acl, err := pan.NewACL(aclInput)
		log.Default().Println("set local ISD policy to", aclInput)

		if err != nil {
			fmt.Println(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.policy = acl
		shttp.DefaultTransport.DialContext = (&shttp.Dialer{
			QuicConfig: nil,
			Policy:     h.policy,
		}).DialContext
		w.WriteHeader(http.StatusOK)
	}
}

// interceptConnect creates a handler that calls the handler connect for the
// CONNECT method and otherwise the next handler.
func interceptConnect(connect, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodConnect {
			connect.ServeHTTP(w, req)
			return
		}
		next.ServeHTTP(w, req)
	}
}

func (h *proxyHandler) handleTunneling(w http.ResponseWriter, req *http.Request) {
	hostAddr, err := pan.ResolveUDPAddr(req.Host)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	session, err := pan.DialQUIC(
		context.Background(),
		netaddr.IPPort{},
		hostAddr,
		h.policy,
		nil,
		req.Host,
		&tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{quicutil.SingleStreamProto},
		},
		nil)
	if err != nil {
		log.Default().Println("error dialing", err.Error())
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	log.Default().Println(session.Conn.RemoteAddr())
	destConn, err := quicutil.NewSingleStream(session)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		// Not expected to happen; the normal ResponseWriter for HTTP/1.x supports
		// this and we're not serving HTTP/2 here (no HTTPS, thus HTTP/2 is
		// disabled).
		panic(fmt.Sprintf("Hijacking not supported for %#v", w))
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	go transfer(destConn, clientConn)
	go transfer(clientConn, destConn)
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

// demunge reverts the host name to a proper SCION address, from the format
// that had been entered in the browser.
func demunge(host string) string {
	parts := mungedScionAddr.FindStringSubmatch(host)
	if parts != nil {
		// directly apply mangling as in pan.MangleSCIONAddr
		return fmt.Sprintf("[%s-%s,%s]",
			parts[mungedScionAddrIAIndex],
			strings.ReplaceAll(parts[mungedScionAddrASIndex], "_", ":"),
			parts[mungedScionAddrHostIndex],
		)
	}
	return host
}

// loadHosts parses /etc/hosts and /etc/scion/hosts looking for SCION host addresses.
// copied/simplified from pkg/pan/hostsfile.go
func loadHosts() []string {
	h1 := loadHostsFile("/etc/hosts")
	h2 := loadHostsFile("/etc/scion/hosts")
	return append(h1, h2...)
}

func loadHostsFile(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	return parseHostsFile(file)
}

// parseHostsFile, copied/simplified from pkg/pan/hostsfile.go
func parseHostsFile(file *os.File) []string {
	hosts := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// ignore comments
		cstart := strings.IndexRune(line, '#')
		if cstart >= 0 {
			line = line[:cstart]
		}

		// cut into fields: address name1 name2 ...
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.Contains(fields[0], ",") { // looks like SCION
			hosts = append(hosts, fields[1:]...)
		}
	}
	return hosts
}
