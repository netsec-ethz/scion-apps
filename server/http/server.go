package shttp

import (
	"context"
	"net"
	"time"

	quic "github.com/scionproto/scion/go/lib/snet/squic"
)

// Server is a  SCION HTTP server
type Server struct {
	Addr    snet.Addr
	network snet.SCIONNetwork

	Handler Handler

	//Timeouts
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

// ListenAndServe creates a HTTP server and listens to requests
func ListenAndServe(addr snet.Addr, handler Handler) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServe()
}

func (srv *Server) Close() error
func (srv *Server) ListenAndServe() error
func (srv *Server) RegisterOnShutdown(f func())
func (srv *Server) Serve(l net.Listener) error
func (srv *Server) SetKeepAlivesEnabled(v bool)
func (srv *Server) Shutdown(ctx context.Context) error

func (srv *Server) Serve(l net.Listener) error {
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

func (srv *Server) ListenAndServe() error {
	ln, err := quic.ListenScion(nil, srv.laddr)
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}
