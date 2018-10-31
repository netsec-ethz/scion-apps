package shttp

import (
	"net/http"
	"testing"
)

func TestWriteHeaderOnlyOnce(t *testing.T) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           conn,
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}
	rw.WriteHeader(200)
	rw.WriteHeader(404)

	if !rw.Header().Get("Status-Code") != 200 {
		t.Errorf("Status-Code changed after call to WriteHeader")
	}

}

func TestCanAddHeaders(t *testing) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           conn,
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}

	len := len(rw.Header())
	rw.Header().Add("Custom-Header", "value")
	rw.WriteHeader(200)
	if len(rw.sendHeader) != len+1 {
		t.Errorf("Failed to add header")
	}
}

func TestCannotAddHeadersAfterWriteHeader(t *testing) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           conn,
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}

	rw.WriteHeader(200)
	len := len(rw.Header())

	// this should add to copy of sendHeader -> no aliasing
	rw.Header().Add("Custom-Header", "value")
	if len(rw.sendHeader) != len {
		t.Errorf("Header added after call to WriteHeader")
	}
}

func TestNotLeakAlias(t *testing) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           conn,
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}

	// get a handle of header
	h := rw.Header()

	// this should assign a copy of handlerHeader to sendHeader -> no aliasing
	rw.WriteHeader(200)
	len := len(rw.Header())

	// try to add header via handle
	h.Add("Custom-Header", "value")
	if len(rw.sendHeader) != len {
		t.Errorf("Possible to write header after call to WriteHeader via alias handle")
	}
}
