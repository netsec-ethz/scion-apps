package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

var (
	serverAddress snet.Addr
	remote        snet.Addr
	sciond        = flag.String("sciond", "", "Path to sciond socket")
	dispatcher    = flag.String("dispatcher", "/run/shm/dispatcher/default.sock", "Path to dispatcher socket")
)

type ScionServer struct {
	Addr    snet.Addr
	network snet.SCIONNetwork

	//Timeouts
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

func (srv *ScionServer) Close() error
func (srv *ScionServer) ListenAndServe() error
func (srv *ScionServer) RegisterOnShutdown(f func())
func (srv *ScionServer) Serve(l net.Listener) error
func (srv *ScionServer) SetKeepAlivesEnabled(v bool)
func (srv *ScionServer) Shutdown(ctx context.Context) error

func (srv *ScionServer) Serve(l net.Listener) error {
	if fn := testHookServerServe; fn != nil {
		fn(srv, l) // call hook with unwrapped listener
	}

	l = &onceCloseListener{Listener: l}
	defer l.Close()

	if err := srv.setupHTTP2_Serve(); err != nil {
		return err
	}

	if !srv.trackListener(&l, true) {
		return ErrServerClosed
	}
	defer srv.trackListener(&l, false)

	var tempDelay time.Duration     // how long to sleep on accept failure
	baseCtx := context.Background() // base is always background, per Issue 16220
	ctx := context.WithValue(baseCtx, ServerContextKey, srv)
	for {
		rw, e := l.Accept()
		if e != nil {
			select {
			case <-srv.getDoneChan():
				return ErrServerClosed
			default:
			}
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				srv.logf("http: Accept error: %v; retrying in %v", e, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return e
		}
		tempDelay = 0
		c := srv.newConn(rw)
		c.setState(c.rwc, StateNew) // before Serve can return
		go c.serve(ctx)
	}
}

func (srv *scionServer) ListenAndServe() error {
	ln, err := squic.ListenScion(nil, srv.laddr)
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}

// helloHandler writes a greeting
type helloHandler struct{}

func (h helloHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World")
}

// main starts serving the web application
func main() {
	srv := &http.Server{
		Addr:    "localhost:8080",
		Handler: helloHandler{},
	}
	srv.ListenAndServe()
}
