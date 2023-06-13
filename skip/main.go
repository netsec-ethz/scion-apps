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
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/gorilla/handlers"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
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

	transport, dialer := shttp.NewTransport(nil, nil)

	proxy := &proxyHandler{
		transport: transport,
	}
	tunnelHandler := &tunnelHandler{
		dialer: dialer,
	}
	policyHandler := &policyHandler{
		output: dialer,
	}

	mux := http.NewServeMux()
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/skip.pac", handleWPAD)
	apiMux.HandleFunc("/scionHosts", handleHostListRequest)
	apiMux.HandleFunc("/r", handleRedirectBackOrError)

	apiMux.HandleFunc("/resolve", handleHostResolutionRequest)
	apiMux.Handle("/setPolicy", policyHandler)

	mux.Handle("localhost/", apiMux)
	if bindAddress.IP != nil {
		mux.Handle(bindAddress.IP.String(), apiMux)
	}
	mux.Handle("/", proxy) // everything else

	handler := interceptConnect(tunnelHandler, mux)

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
		fmt.Println("verbose: ", "error executing template")
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
	for i := 1; i < len(scionHost); i++ {
		buf.WriteString("\n" + scionHost[i])
	}
	_, _ = w.Write(buf.Bytes())
}

func handleRedirectBackOrError(w http.ResponseWriter, req *http.Request) {

	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// We need this here, it's required for redirecting properly
	// We may set localhost here but this would stop us from
	// running one skip for multiple clients later...
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	q := req.URL.Query()

	urls, ok := q["url"]
	if !ok || len(urls) != 1 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	url, err := url.Parse(urls[0])
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	hostPort := url.Host + ":0"

	w.Header().Set("Location", url.String())
	_, err = pan.ResolveUDPAddr(req.Context(), hostPort)
	if err != nil {
		fmt.Println("verbose: ", err.Error())
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, req, url.String(), http.StatusMovedPermanently)
}

// handleHostResolutionRequest parses requests in the form: /resolve?host=XXX
// If the PAN lib cannot resolve the host, it sends back an empty response.
func handleHostResolutionRequest(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	buf := &bytes.Buffer{}
	q := req.URL.Query()
	hosts, ok := q["host"]
	if !ok || len(hosts) > 1 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	hostPort := hosts[0] + ":0"

	res, err := pan.ResolveUDPAddr(req.Context(), hostPort)
	if err != nil {
		fmt.Println("verbose: ", err.Error())
		ok := errors.As(err, &pan.HostNotFoundError{})
		if !ok {
			http.Error(w, "Internal error", http.StatusInternalServerError)
		}
		return
	}
	buf.WriteString(strings.TrimRight(res.String(), ":0"))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func isSCIONEnabled(ctx context.Context, host string) (bool, error) {
	_, err := pan.ResolveUDPAddr(ctx, host)
	if err != nil {
		fmt.Println("verbose: ", err.Error())
		ok := errors.As(err, &pan.HostNotFoundError{})
		if !ok {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

type policyHandler struct {
	output interface{ SetPolicy(pan.Policy) }
}

func (h *policyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if req.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		fmt.Println("verbose: ", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var acl pan.ACL
	err = acl.UnmarshalJSON(body)
	if err != nil {
		fmt.Println("verbose: ", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.output.SetPolicy(&acl)
	fmt.Println("verbose: ", "ACL policy = ", acl.String())
	w.WriteHeader(http.StatusOK)
}

type proxyHandler struct {
	transport http.RoundTripper
}

func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	host := demunge(req.Host)
	req.Host = host
	req.URL.Host = host

	resp, err := h.transport.RoundTrip(req)
	if err != nil {
		fmt.Println("verbose: ", err.Error())
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

type tunnelHandler struct {
	dialer *shttp.Dialer
}

func (h *tunnelHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	hostPort := req.Host
	var destConn net.Conn
	var err error
	enabled, _ := isSCIONEnabled(req.Context(), hostPort)
	if !enabled {
		// CONNECT via TCP/IP
		destConn, err = net.DialTimeout("tcp", req.Host, 10*time.Second)
	} else {
		// CONNECT via SCION
		destConn, err = h.dialer.DialContext(context.Background(), "", req.Host) //nolint:contextcheck
	}
	if err != nil {
		fmt.Println("verbose: ", err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Println("verbose: ", "Remote addr = ", destConn.RemoteAddr())
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		// Not expected to happen; the normal ResponseWriter for HTTP/1.x supports
		// this and we're not serving HTTP/2 here (no HTTPS, thus HTTP/2 is
		// disabled).
		panic(fmt.Sprintf("Hijacking not supported for %#v", w))
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		fmt.Println("verbose: ", "verbose: ", err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
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
