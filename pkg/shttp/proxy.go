package shttp

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http/httputil"
	"net/url"
)

// Proxies the incoming HTTP/1.1 request to the configured remote
// creating a new SCION HTTP/3 request
func NewSingleSCIONHostReverseProxy(remote string, cliTLSCfg *tls.Config) *httputil.ReverseProxy {
	// Enforce HTTPS, otherwise it cannot be parsed to URL
	sUrl := MangleSCIONAddrURL(fmt.Sprintf("https://%s", remote))
	targetURL, err := url.Parse(sUrl)
	if err != nil {
		log.Fatalf("Failed to parse SCION URL %s", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = NewRoundTripper(cliTLSCfg, nil)
	return proxy
}
