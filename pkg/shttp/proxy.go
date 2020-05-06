package shttp

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// Proxies the incoming HTTP/1.1 request to the configured remote
// creating a new SCION HTTP/3 request
func NewSingleSCIONHostReverseProxy(remote string, cliTLSCfg *tls.Config) http.HandlerFunc {
	client := &http.Client{
		Transport: NewRoundTripper(cliTLSCfg, nil),
	}

	handler := func(wr http.ResponseWriter, r *http.Request) {
		// Enforce HTTPS
		baseStr := "%s%s"
		if !strings.Contains(remote, "https://") {
			baseStr = "https://%s%s"
		}
		url := fmt.Sprintf(baseStr, remote, r.URL.Path)
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

		resp, err := client.Do(req)

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

	return handler
}
