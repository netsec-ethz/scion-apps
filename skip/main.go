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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/handlers"
	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/daemon"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
	"github.com/netsec-ethz/scion-apps/pkg/shttp"
)

var (
	mungedScionAddr = regexp.MustCompile(`^(\d+)-([_\dA-Fa-f]+)-(.*)$`)
	localIA         addr.IA
)

const (
	mungedScionAddrIAIndex   = 1
	mungedScionAddrASIndex   = 2
	mungedScionAddrHostIndex = 3
)

// This is at the moment just for presentation purposes and needs to be
// rewritten in the end...
var pathStats = PathUsageStats{
	data: make(map[string]*PathUsage),
}

type PathUsage struct {
	Received int64
	Path     string
	Strategy string
	Domain   string
}

type PathUsageStats struct {
	sync.Mutex
	// For simplicity before the Hotnets Demo: We assume that here is one path used per domain
	data map[string]*PathUsage
}

//go:embed skip.pac
var skipPAC string
var skipPACtemplate = template.Must(template.New("skip.pac").Parse(skipPAC))

type skipPACTemplateParams struct {
	ProxyAddress string
	SCIONHosts   []string
}

func main() {
	var bindAddress *net.TCPAddr
	var err error
	kingpin.Flag("bind", "Address to bind on").Default("localhost:8888").TCPVar(&bindAddress)
	kingpin.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	daemon, err := findSciond(ctx)
	if err != nil {
		fmt.Printf("Cannot connect to SCION Daemon: %s\n", err)
	}
	localIA, err = daemon.LocalIA(ctx)
	if err != nil {
		fmt.Printf("Parsing local IA: %s\n", err)
	// XXX(JordiSubira): The SCIONExperimental version is intended to be used
	// under any contricated network deployment. Keep in mind, that the remote
	// server should also supported.
	// If trying to contact a server without this version, the version on the
	// client should be consistent with it.
	// TODO(JordiSubira): Do this configurable.
	quicCfg := &quic.Config{
		Versions: []quic.VersionNumber{quicutil.VersionSCIONExperimental},
	}

	transport, dialer := shttp.NewTransport(quicCfg, nil)

	proxy := &proxyHandler{
		transport: transport,
		dialer:    dialer,
	}
	tunnelHandler := &tunnelHandler{
		dialer: dialer,
	}
	policyHandler := &policyHandler{
		output: dialer,
	}
	errorHandler := &errorHandler{
		dialer: dialer,
	}

	mux := http.NewServeMux()
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/skip.pac", handleWPAD)
	apiMux.HandleFunc("/scionHosts", handleHostListRequest)
	apiMux.HandleFunc("/pathUsage", handlePathUsageRequest)
	apiMux.HandleFunc("/r", handleRedirectBackOrError)

	apiMux.HandleFunc("/resolve", handleHostResolutionRequest)
	apiMux.Handle("/setPolicy", policyHandler)
	apiMux.Handle("/error", errorHandler)

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

func findSciond(ctx context.Context) (daemon.Connector, error) {
	address, ok := os.LookupEnv("SCION_DAEMON_ADDRESS")
	if !ok {
		address = daemon.DefaultAPIAddress
	}
	sciondConn, err := daemon.NewService(address).Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to SCIOND at %s (override with SCION_DAEMON_ADDRESS): %w", address, err)
	}
	return sciondConn, nil
}

func handlePathUsageRequest(w http.ResponseWriter, req *http.Request) {
	data := make([]*PathUsage, 0)
	for _, v := range pathStats.data {
		data = append(data, v)
	}
	j, err := json.Marshal(data)
	if err != nil {
		fmt.Println("verbose: ", "error serializing path statistics")
		http.Error(w, "error serializing path statistics", 500)
		return
	}
	_, _ = w.Write(j)
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

type errorHandler struct {
	dialer *shttp.Dialer
}

func (h *errorHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	_, err = h.dialer.DialContext(ctx, "", hostPort)

	buf := &bytes.Buffer{}

	w.WriteHeader(http.StatusOK)
	buf.WriteString(err.Error())
	_, _ = w.Write(buf.Bytes())

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

	// Check if acl
	var policy pan.Policy
	var acl pan.ACL
	err = acl.UnmarshalJSON(body)
	policy = &acl
	if err != nil {
		fmt.Println("verbose: not ACL format, trying Sequence format out")
		var str string
		err2 := json.Unmarshal(body, &str)
		if err2 != nil {
			fmt.Println("verbose: ", err.Error(), err2.Error())
			http.Error(w, err.Error()+" "+err2.Error(), http.StatusBadRequest)
			return
		}
		seqStr, err2 := parseShowPathToSeq(str)
		if err2 != nil {
			fmt.Println("verbose: ", err.Error(), err2.Error())
			http.Error(w, err.Error()+" "+err2.Error(), http.StatusBadRequest)
			return
		}
		sequence, err2 := pan.NewSequence(seqStr)
		if err2 != nil {
			fmt.Println("verbose: ", err.Error(), err2.Error())
			http.Error(w, err.Error()+" "+err2.Error(), http.StatusBadRequest)
			return
		}
		policy = sequence
	}
	h.output.SetPolicy(policy)
	fmt.Println("verbose: ", "Policy = ", string(body))
	w.WriteHeader(http.StatusOK)
}

type proxyHandler struct {
	transport http.RoundTripper
	dialer    *shttp.Dialer
}

func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	host := demunge(req.Host)
	req.Host = host
	req.URL.Host = host

	// TODO(JordiSubira): This code snippet needs to be adapted and polished
	// to add path usage information for HTTP(no S) connections.
	//
	// domain := req.URL.Hostname()
	// policy := h.dialer.Policy
	// sequence, ok := policy.(pan.Sequence)
	// var pathUsage *PathUsage
	// if ok {
	// 	pu, ok := pathStats.data[domain]
	// 	if !ok {
	// 		pathUsage = &PathUsage{
	// 			Received: 0,
	// 			Strategy: "Geofenced", // TODO: This may be configured by the user
	// 			Path:     sequenceToPath(sequence),
	// 			Domain:   domain,
	// 		}

	// 		pathStats.Lock()
	// 		pathStats.data[domain] = pathUsage
	// 		pathStats.Unlock()
	// 	} else {
	// 		pathUsage = pu
	// 		pathUsage.Path = sequenceToPath(sequence)
	// 	}
	// }

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
	var pathF func() *pan.Path
	if !enabled {
		// CONNECT via TCP/IP
		destConn, err = net.DialTimeout("tcp", req.Host, 10*time.Second)
	} else {
		// CONNECT via SCION
		ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
		defer cancel()
		destConn, err = h.dialer.DialContext(ctx, "", req.Host)
		if err != nil {
			fmt.Println("verbose: ", err.Error())
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if panConn, ok := destConn.(*quicutil.SingleStream); ok {
			pathF = panConn.GetPath
			fmt.Printf("%s\n", pathToShortPath(pathF()))
		}
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
		fmt.Println("verbose: ", err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	// This is at the moment just for presentation purposes and needs to be
	// rewritten in the end...
	go transfer(destConn, clientConn, nil, req.URL.Hostname()) // We just count the received bytes for now
	go transfer(clientConn, destConn, pathF, req.URL.Hostname())

}

func pathToShortPath(path *pan.Path) string {
	if path == nil || path.Metadata == nil || len(path.Metadata.Interfaces) == 0 {
		// TODO(JordiSubira): At the moment, treat all these cases as empty path.
		// For visualization purposes we show the local IA.
		return localIA.String()
	}
	b := &strings.Builder{}
	intf := path.Metadata.Interfaces[0]
	fmt.Fprintf(b, "%s", intf.IA)
	for i := 1; i < len(path.Metadata.Interfaces)-1; i += 2 {
		inIntf := path.Metadata.Interfaces[i]
		fmt.Fprintf(b, " -> %s ", inIntf.IA)
	}
	intf = path.Metadata.Interfaces[len(path.Metadata.Interfaces)-1]
	fmt.Fprintf(b, " -> %s", intf.IA)
	return b.String()
}

func transfer(dst io.WriteCloser, src io.ReadCloser, pathF func() *pan.Path, domain string) {
	defer dst.Close()
	defer src.Close()
	buf := make([]byte, 1024)
	var written int64

	var pathUsage *PathUsage
	if pathF != nil {
		path := pathF()
		pu, ok := pathStats.data[domain]
		if !ok {
			pathUsage = &PathUsage{
				Received: 0,
				Strategy: "Shortest Path", // TODO: This may be configured by the user
				Path:     pathToShortPath(path),
				Domain:   domain,
			}

			pathStats.Lock()
			pathStats.data[domain] = pathUsage
			pathStats.Unlock()
		} else {
			pathUsage = pu
			pathUsage.Path = pathToShortPath(path)
		}
	}

	for {
		nr, er := src.Read(buf)
		if pathUsage != nil {
			pathUsage.Received += int64(nr)
		}
		if pathF != nil && pathUsage != nil {
			pathUsage.Path = pathToShortPath(pathF())
		}

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

type step struct {
	IA      addr.IA
	Ingress int
	Egress  int
}

func stringToStep(s []string) (step, error) {
	var step step
	var err error
	if len(s) != 3 {
		return step, fmt.Errorf("wrong size %d != 3", len(s))
	}
	step.IA, err = addr.ParseIA(s[1])
	if err != nil {
		return step, err
	}
	step.Ingress, err = strconv.Atoi(s[0])
	if err != nil {
		return step, err
	}
	step.Egress, err = strconv.Atoi(s[2])
	if err != nil {
		return step, err
	}
	return step, nil
}

type steps []step

func (s steps) ToSequenceStr() string {
	// 19-ffaa:1:f5c 1>370 19-ffaa:0:1303 1>5 19-ffaa:0:1301 3>5 18-ffaa:0:1201 8>1 18-ffaa:0:1206 128>1 18-ffaa:1:feb
	// 19-ffaa:1:f5c#1 19-ffaa:0:1303#370,1 19-ffaa:0:1301#5,3 18-ffaa:0:1201#5,8 18-ffaa:0:1206#1,128 18-ffaa:1:feb#1
	b := &strings.Builder{}
	for i, step := range s {
		if i == 0 {
			fmt.Fprintf(b, "%s #%d", step.IA.String(), step.Egress)
			continue
		}
		if i == len(s)-1 {
			fmt.Fprintf(b, " %s #%d", step.IA.String(), step.Ingress)
			continue
		}
		fmt.Fprintf(b, " %s #%d,%d", step.IA.String(), step.Ingress, step.Egress)
	}
	return b.String()
}

func parseShowPaths(s string) (steps, error) {
	iaInterfaces := strings.Split(s, ">")

	if len(iaInterfaces) < 2 {
		return nil, fmt.Errorf("iaInterfaces length %d < 2", len(iaInterfaces))
	}

	steps := make([]step, len(iaInterfaces))
	var err error

	// Add special value to the beginning and end of the path
	iaInterfaces[0] = "0 " + iaInterfaces[0]
	iaInterfaces[len(steps)-1] = iaInterfaces[len(steps)-1] + " 0"

	for i := 0; i < len(steps); i++ {
		steps[i], err = stringToStep(strings.Split(iaInterfaces[i], " "))
		if err != nil {
			return nil, err
		}
	}

	for _, step := range steps {
		fmt.Printf("%s %d %d, ", step.IA.String(), step.Ingress, step.Egress)
	}
	return steps, nil
}

func parseShowPathToSeq(s string) (string, error) {
	steps, err := parseShowPaths(s)
	if err != nil {
		return "", err
	}
	return steps.ToSequenceStr(), nil
}
