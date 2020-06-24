// Copyright 2017 ETH Zurich
// Copyright 2018 ETH Zurich, Anapaya Systems
// Copyright 2019 ETH Zurich
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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/quictrace"

	"strings"

	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/snet"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
	"github.com/netsec-ethz/scion-apps/pkg/mpsquic"
)

const (
	DefaultInterval = 1 * time.Second
	DefaultTimeout  = 2 * time.Second
	MaxPings        = 1 << 16
	ReqMsg          = "boing?"
	ReplyMsg        = "boing!"
	ModeServer      = "server"
	ModeClient      = "client"
	nextProto       = "boingboing"

	errorNoError quic.ErrorCode = 0x100
	errorTimeout quic.ErrorCode = 0x200
)

var (
	remote snet.UDPAddr
	file   = flag.String("file", "",
		"File containing the data to send, optional to test larger data (only client)")
	interactive = flag.Bool("i", false, "Interactive mode")
	mode        = flag.String("mode", ModeClient, "Run in "+ModeClient+" or "+ModeServer+" mode")
	count       = flag.Int("count", 0,
		fmt.Sprintf("Number of boings, between 0 and %d; a count of 0 means infinity", MaxPings))
	timeout = flag.Duration("timeout", DefaultTimeout,
		"Timeout for the boing response")
	interval = flag.Duration("interval", DefaultInterval, "time between boings")
	trace    = flag.String("trace", "", "Directory for quic traces. Enables tracing of the QUIC connection")
	port     = flag.Int("port", 0, "(Mandatory for server) Server port")
	fileData []byte

	// No way to extract error code from error returned after closing session in quic-go.
	// c.f. https://github.com/lucas-clemente/quic-go/issues/2441
	// Workaround by string comparison with known formated error string.
	errorNoErrorString = fmt.Sprintf("Application error %#x", uint64(errorNoError))
)

func init() {
	flag.Var(&remote, "remote", "(Mandatory for clients) address to connect to")
}

func main() {
	validateFlags()
	switch *mode {
	case ModeClient:
		paths, err := choosePaths(remote.IA, *interactive)
		if err != nil {
			LogFatal("No paths available to remote destination")
		}
		c := &client{}
		setSignalHandler(c)
		c.run(&remote, paths)
	case ModeServer:
		server{}.run()
	}
}

func validateFlags() {
	flag.Parse()
	if *mode != ModeClient && *mode != ModeServer {
		LogFatal("Unknown mode, must be either '" + ModeClient + "' or '" + ModeServer + "'")
	}
	if *mode == ModeClient {
		if remote.Host == nil {
			LogFatal("Missing remote address")
		}
		if remote.Host.Port == 0 {
			LogFatal("Missing remote port")
		}
	} else {
		if *port == 0 {
			LogFatal("Missing server port")
		}
	}
	if *count < 0 || *count > MaxPings {
		LogFatal("Invalid count", "min", 0, "max", MaxPings, "actual", *count)
	}
	if *file != "" {
		if *mode == ModeClient {
			var err error
			fileData, err = ioutil.ReadFile(*file)
			if err != nil {
				LogFatal("Could not read data file")
			}
		} else {
			log.Info("file argument is ignored for mode " + ModeServer)
		}
	}
	if *trace != "" {
		if stat, err := os.Stat(*trace); os.IsNotExist(err) || !stat.IsDir() {
			LogFatal("-trace must point to a directory")
		}
	}
}

func LogFatal(msg string, a ...interface{}) {
	log.Crit(msg, a...)
	os.Exit(1)
}

type message struct {
	BoingBoing string
	Data       []byte
	Timestamp  int64
}

func requestMsg() *message {
	return &message{
		BoingBoing: ReqMsg,
		Data:       fileData,
		Timestamp:  time.Now().UnixNano(),
	}
}

func replyMsg(request *message) *message {
	return &message{
		ReplyMsg,
		request.Data,
		request.Timestamp,
	}
}

func (m *message) len() int {
	return len(m.BoingBoing) + len(m.Data) + 8
}

type quicStream struct {
	qstream quic.Stream
	encoder *gob.Encoder
	decoder *gob.Decoder
}

func newQuicStream(qstream quic.Stream) *quicStream {
	return &quicStream{
		qstream,
		gob.NewEncoder(qstream),
		gob.NewDecoder(qstream),
	}
}

func (qs quicStream) WriteMsg(msg *message) error {
	return qs.encoder.Encode(msg)
}

func (qs quicStream) ReadMsg() (*message, error) {
	var msg message
	err := qs.decoder.Decode(&msg)
	if err != nil {
		return nil, err
	}
	return &msg, err
}

type client struct {
	*quicStream
	qsess quic.Session
}

// run dials to a remote SCION address and repeatedly sends boing? messages
// while receiving boing! messages. For each successful boing?-boing!, a message
// with the round trip time is printed.
func (c *client) run(remote *snet.UDPAddr, paths []snet.Path) {

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{nextProto},
	}

	var tracer quictrace.Tracer
	if *trace != "" {
		tracer = quictrace.NewTracer()
	}
	quicConf := &quic.Config{
		QuicTracer: tracer,
	}
	var err error
	if remote.IA == appnet.DefNetwork().IA {
		// XXX(matzf) mpsquic does not properly handle destination in same AS. Too many places assume
		// an existing path. Easy fallback, use normal appquic.
		c.qsess, err = appquic.DialAddr(remote, "host:0", tlsConf, quicConf)
	} else {
		c.qsess, err = mpsquic.Dial(remote, "host:0", paths, tlsConf, quicConf)
	}
	if err != nil {
		LogFatal("Unable to dial", "err", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	qstream, err := c.qsess.OpenStreamSync(ctx)
	cancel()
	if err != nil {
		LogFatal("quic OpenStream failed", "err", err)
	}
	c.quicStream = newQuicStream(qstream)
	log.Debug("Quic stream opened", "remote", remote)
	go func() {
		defer log.HandlePanic()
		c.send()
	}()
	c.read()

	if tracer != nil {
		err := exportTraces(tracer, *trace)
		if err != nil {
			log.Debug("Error while exporting QUIC trace", "err", err)
		}
	}

	log.Info("Client run completed")
}

func (c *client) Close() error {
	var err error
	if c.quicStream != nil && c.qstream != nil {
		err = c.qstream.Close()
	}
	if err == nil && c.qsess != nil {
		// Note closing the session here is fine since we know that all the traffic went through.
		// If you are not sure that this is the case you should probably not close the session.
		// E.g. if you are just sending something to a server and closing the session immediately
		// it might be that the server does not see the message.
		// See also: https://github.com/lucas-clemente/quic-go/issues/464
		_ = c.qsess.CloseWithError(errorNoError, "")
	}
	return err
}

func (c client) send() {
	if fileData == nil {
		fileData = []byte(strings.Repeat("A", 1e5))
	}
	for i := 0; i < *count || *count == 0; i++ {
		if i != 0 && *interval != 0 {
			time.Sleep(*interval)
		}

		// Send boing? message to destination
		reqMsg := requestMsg()

		err := c.qstream.SetWriteDeadline(time.Now().Add(*timeout))
		if err != nil {
			LogFatal("Unable to set write deadline", "err", err)
		}
		err = c.WriteMsg(reqMsg)
		if err != nil {
			log.Error("Unable to write", "err", err)
			c.qstream.CancelRead(errorTimeout)
			c.qstream.Close()
			return
		}
		log.Info("Wrote message", "server", &remote, "len", reqMsg.len(), "seq", i)

		// After sending each request, set a ReadDeadline for the expected reply on the stream
		err = c.qstream.SetReadDeadline(time.Now().Add(*timeout))
		if err != nil {
			LogFatal("Unable to set read deadline", "err", err)
		}
	}
	fmt.Println("-----------------------------Client done sending.-----------------------------")
}

func (c client) read() {
	// Receive boing! message (with final timeout)
	for i := 0; i < *count || *count == 0; i++ {
		msg, err := c.ReadMsg()
		after := time.Now()
		if err != nil {
			if err == io.EOF || err.Error() == errorNoErrorString {
				log.Info("Quic session ended")
			} else {
				log.Error("Unable to read", "err", err)
			}
			break
		}
		if msg.BoingBoing != ReplyMsg {
			log.Error("Received wrong message", "expected", ReplyMsg, "actual", msg.BoingBoing)
		} else if !bytes.Equal(msg.Data, fileData) {
			log.Error("Received different data than sent.")
		} else {
			before := time.Unix(0, int64(msg.Timestamp))
			elapsed := after.Sub(before).Round(time.Microsecond)
			log.Info("Received reply", "server", &remote, "len", msg.len(), "seq", i, "RTT", elapsed)
		}
	}
	fmt.Println("-----------------------------Client done receiving.-----------------------------")
}

type server struct {
}

// run listens on a SCION address and replies to any boing? message.
// On any error, the server exits.
func (s server) run() {
	// Listen on SCION address
	qsock, err := mpsquic.ListenPort(
		uint16(*port),
		&tls.Config{
			Certificates: appquic.GetDummyTLSCerts(),
			NextProtos:   []string{nextProto},
		},
		nil,
	)
	if err != nil {
		LogFatal("Unable to listen", "err", err)
	}
	log.Info("Listening", "local", qsock.Addr())
	for {
		qsess, err := qsock.Accept(context.Background())
		if err != nil {
			log.Error("Unable to accept quic session", "err", err)
			// Accept failing means the socket is unusable.
			break
		}
		log.Info("Quic session accepted", "src", qsess.RemoteAddr())
		go func() {
			defer log.HandlePanic()
			s.handleClient(qsess)
		}()
	}
}

func (s server) handleClient(qsess quic.Session) {
	qstream, err := qsess.AcceptStream(context.Background())
	if err != nil {
		log.Error("Unable to accept quic stream", "err", err)
		return
	}

	qs := newQuicStream(qstream)
	for {
		// Receive boing? message
		msg, err := qs.ReadMsg()
		if err != nil {
			if err == io.EOF || err.Error() == errorNoErrorString {
				log.Info("Quic session ended", "src", qsess.RemoteAddr())
			} else {
				log.Error("Unable to read", "err", err)
			}
			break
		}

		dt := time.Since(time.Unix(0, msg.Timestamp)).Round(time.Microsecond)
		log.Info("Received message", "client", qsess.RemoteAddr(), "len", msg.len(), "dt", dt)

		// Send boing! message
		_ = qs.qstream.SetWriteDeadline(time.Now().Add(*timeout))
		replyMsg := replyMsg(msg)
		err = qs.WriteMsg(replyMsg)
		if err != nil {
			log.Error("Unable to write", "err", err)
			break
		}
		log.Info("Wrote reply", "client", qsess.RemoteAddr(), "len", replyMsg.len())
	}

	err = qstream.Close()
	if err == nil {
		_ = qsess.CloseWithError(errorNoError, "")
	}
}

func setSignalHandler(closer io.Closer) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer log.HandlePanic()
		<-c
		closer.Close()
	}()
}

func exportTraces(tracer quictrace.Tracer, dir string) error {
	traces := tracer.GetAllTraces()
	i := 0
	for _, trace := range traces {
		f, err := os.Create(path.Join(dir, fmt.Sprintf("trace_%d.qtr", i)))
		if err != nil {
			return err
		}
		if _, err := f.Write(trace); err != nil {
			return err
		}
		log.Debug("Wrote QUIC trace file", "path", f.Name())
		i += 1
	}
	return nil
}
