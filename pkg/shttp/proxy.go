package shttp

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// Args to create proxy instance
type ProxyArgs struct {
	Direction string
	Local     string
	Remote    string
	TlsCert   *string
	TlsKey    *string
}

// Can be overwritte by api calls
type ProxyConfig struct {
	Remote string `json:"remote"`
}

type SCIONHTTPProxy struct {
	config    *ProxyConfig
	client    *http.Client
	direction string
	local     string
	// Don't verify the server's cert, as we are not using the TLS PKI.
	cert      *tls.Certificate
	cliTLSCfg *tls.Config
	srvTLSCfg *tls.Config
}

// Start listen according to the passed direction
func (s SCIONHTTPProxy) Start() {
	if s.direction == "toScion" {
		s.client = &http.Client{
			Transport: NewRoundTripper(s.cliTLSCfg, nil),
		}

		http.HandleFunc("/__api/setconfig", s.setProxyConfig)
		http.HandleFunc("/", s.proxyToScion)
		log.Fatal(http.ListenAndServe(s.local, nil))
	} else {
		mux := http.NewServeMux()
		mux.HandleFunc("/__api/setconfig", s.setProxyConfig)
		mux.HandleFunc("/", s.proxyFromScion)
		log.Fatal(ListenAndServe(s.local, mux, s.srvTLSCfg))
	}
}

// Create new SCION HTTP Proxy instance
// TLS cert/key can be passed optionally
func NewSCIONHTTPProxy(args ProxyArgs) (*SCIONHTTPProxy, error) {

	config := &ProxyConfig{
		Remote: args.Remote,
	}

	proxy := &SCIONHTTPProxy{
		config:    config,
		direction: args.Direction,
		local:     args.Local,
	}

	// Allow use of external certificates instead of dummy certs
	if args.TlsCert != nil && args.TlsKey != nil {
		cert, err := tls.LoadX509KeyPair(*args.TlsCert, *args.TlsKey)
		if err != nil {
			return nil, err
		}

		proxy.cliTLSCfg = &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3-24"}}
		proxy.srvTLSCfg = &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3-24"}}
		proxy.srvTLSCfg.Certificates = []tls.Certificate{cert}
		proxy.cert = &cert
	}

	return proxy, nil
}

// API Endpoint, updates the proxy config to set new values
// Currently, only changing the remote is possible
func (s SCIONHTTPProxy) setProxyConfig(wr http.ResponseWriter, r *http.Request) {
	var newConfig ProxyConfig

	// Try to decode the request body into the struct. If there is an error,
	// respond to the client with the error message and a 400 status code.
	err := json.NewDecoder(r.Body).Decode(&newConfig)
	if err != nil {
		http.Error(wr, err.Error(), http.StatusBadRequest)
		return
	}

	log.Println("Perform config update")
	s.config = &newConfig
}

// Proxies the incoming HTTP/1.1 request to the configured remote
// creating a new SCION HTTP/3 request
func (s SCIONHTTPProxy) proxyToScion(wr http.ResponseWriter, r *http.Request) {

	// Enforce HTTPS
	baseStr := "%s%s"
	if !strings.Contains(s.config.Remote, "https://") {
		baseStr = "https://%s%s"
	}
	url := fmt.Sprintf(baseStr, s.config.Remote, r.URL.Path)
	log.Println(fmt.Sprintf("Proxy to SCION, do %s request to url %s", r.Method, url))

	req, err := http.NewRequest(r.Method, url, nil)
	if err != nil {
		log.Println("request creation failed: ", err)
		http.Error(wr, err.Error(), http.StatusInternalServerError)
		return
	}

	for k, v := range r.Header {
		req.Header.Set(k, v[0])
	}

	resp, err := s.client.Do(req)

	if err != nil {
		log.Println("request failed: ", err)
		http.Error(wr, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		wr.Header().Set(k, v[0])
	}

	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
}

// Proxies the incoming SCION HTTP/3 request to the configured remote
// creating a new HTTP/1.1 request
func (s SCIONHTTPProxy) proxyFromScion(wr http.ResponseWriter, r *http.Request) {
	var resp *http.Response
	var err error
	var req *http.Request
	client := &http.Client{}

	// No HTTPS replacement, because we do not know if the remote
	// uses HTTPS
	remoteUrl := fmt.Sprintf("%s%s", s.config.Remote, r.URL.Path)
	log.Println(fmt.Sprintf("Proxy from SCION, do %s request to url %s", r.Method, remoteUrl))
	req, err = http.NewRequest(r.Method, remoteUrl, nil)

	if err != nil {
		log.Println("request creation failed: ", err)
		http.Error(wr, err.Error(), http.StatusInternalServerError)
		return
	}

	for name, value := range r.Header {
		req.Header.Set(name, value[0])
	}

	resp, err = client.Do(req)

	if err != nil {
		log.Println("request failed: ", err)
		http.Error(wr, err.Error(), http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()
	for k, v := range resp.Header {
		wr.Header().Set(k, v[0])
	}
	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
}
