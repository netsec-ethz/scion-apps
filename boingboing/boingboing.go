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

// Simple application for SCION connectivity using the snet library.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/lucas-clemente/quic-go"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	sd "github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
	"github.com/scionproto/scion/go/lib/spath"
	"strings"

	"github.com/netsec-ethz/scion-apps/lib/mpsquic"
)

const (
	DefaultInterval = 1 * time.Second
	DefaultTimeout  = 2 * time.Second
	MaxPings        = 1 << 16
	ReqMsg          = "boing?"
	ReplyMsg        = "boing!"
	TSLen           = 8
	ModeServer      = "server"
	ModeClient      = "client"
)

var (
	local   snet.Addr
	remote  snet.Addr
	remotes []*snet.Addr
	file    = flag.String("file", "",
		"File containing the data to send, optional to test larger data (only client)")
	interactive = flag.Bool("ibb", false, "Interactive mode")
	id          = flag.String("id", "boingboing", "Element ID")
	mode        = flag.String("mode", ModeClient, "Run in "+ModeClient+" or "+ModeServer+" mode")
	sciond      = flag.String("sciond", "", "Path to sciond socket")
	dispatcher  = flag.String("dispatcher", "", "Path to dispatcher socket")
	count       = flag.Int("count", 0,
		fmt.Sprintf("Number of boings, between 0 and %d; a count of 0 means infinity", MaxPings))
	timeout = flag.Duration("timeoutbb", DefaultTimeout,
		"Timeout for the boing response")
	interval     = flag.Duration("intervalbb", DefaultInterval, "time between boings")
	verbose      = flag.Bool("v", false, "sets verbose output")
	sciondFromIA = flag.Bool("sciondFromIA", false,
		"SCIOND socket path from IA address:ISD-AS")
	fileData []byte
)

func init() {
	flag.Var((*snet.Addr)(&local), "localbb", "(Mandatory) address to listen on")
	flag.Var((*snet.Addr)(&remote), "remotebb", "(Mandatory for clients) address to connect to")
}

func main() {
	os.Setenv("TZ", "UTC")
	log.AddLogConsFlags()
	validateFlags()
	if err := log.SetupFromFlags(""); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s", err)
		flag.Usage()
		os.Exit(1)
	}
	defer log.LogPanicAndExit()
	initNetwork()
	switch *mode {
	case ModeClient:
		c := newClient()
		setSignalHandler(c)
		c.run()
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
		if remote.Host.L4 == nil {
			LogFatal("Missing remote port")
		}
		if remote.Host.L4.Port() == 0 {
			LogFatal("Invalid remote port", "remote port", remote.Host.L4.Port())
		}
	}
	if local.Host == nil {
		LogFatal("Missing local address")
	}
	if *sciondFromIA {
		if *sciond != "" {
			LogFatal("Only one of -sciond or -sciondFromIA can be specified")
		}
		if local.IA.IsZero() {
			LogFatal("-local flag is missing")
		}
		*sciond = sd.GetDefaultSCIONDPath(&local.IA)
	} else if *sciond == "" {
		*sciond = sd.GetDefaultSCIONDPath(nil)
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
}

func LogFatal(msg string, a ...interface{}) {
	log.Crit(msg, a...)
	os.Exit(1)
}

func initNetwork() {
	// Initialize default SCION networking context
	if err := snet.Init(local.IA, *sciond, reliable.NewDispatcherService(*dispatcher)); err != nil {
		LogFatal("Unable to initialize SCION network", "err", err)
	}
	log.Debug("SCION network successfully initialized")
	if err := mpsquic.Init("", ""); err != nil {
		LogFatal("Unable to initialize QUIC/SCION", "err", err)
	}
	log.Debug("QUIC/SCION successfully initialized")
}

type message struct {
	BoingBoing  string
	Data      []byte
	Timestamp int64
}

func requestMsg() *message {
	return &message{
		BoingBoing: ReqMsg,
		Data:     fileData,
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

type mpQuicStream struct {
	qstream quic.Stream
	encoder *gob.Encoder
	decoder *gob.Decoder
}

func newMPQuicStream(qstream quic.Stream) *mpQuicStream {
	return &mpQuicStream{
		qstream,
		gob.NewEncoder(qstream),
		gob.NewDecoder(qstream),
	}
}

func (qs mpQuicStream) WriteMsg(msg *message) error {
	return qs.encoder.Encode(msg)
}

func (qs mpQuicStream) ReadMsg() (*message, error) {
	var msg message
	err := qs.decoder.Decode(&msg)
	if err != nil {
		return nil, err
	}
	return &msg, err
}

type client struct {
	*mpQuicStream
	mpQuic *mpsquic.MPQuic
}

func newClient() *client {
	return &client{}
}

// run dials to a remote SCION address and repeatedly sends boing? messages
// while receiving boing! messages. For each successful boing?-boing!, a message
// with the round trip time is printed.
func (c *client) run() {
	// Needs to happen before DialSCION, as it will 'copy' the remote to the connection.
	// If remote is not in local AS, we need a path!
	c.setupPaths()
	defer c.Close()

	// Connect to remote addresses. Note that currently the SCION library
	// does not support automatic binding to local addresses, so the local
	// IP address needs to be supplied explicitly. When supplied a local
	// port of 0, DialSCION will assign a random free local port.
	mpQuic, err := mpsquic.DialMP(nil, &local, remotes, nil)
	if err != nil {
		LogFatal("Unable to dial", "err", err)
	}
	c.mpQuic = mpQuic

	qstream, err := c.mpQuic.OpenStreamSync()
	if err != nil {
		LogFatal("quic OpenStream failed", "err", err)
	}
	c.mpQuicStream = newMPQuicStream(qstream)
	log.Debug("Quic stream opened", "local", &local, "remote", &remote)
	go func() {
		defer log.LogPanicAndExit()
		c.send()
	}()
	c.read()
}

func (c *client) Close() error {
	var err error
	if c.qstream != nil {
		err = c.qstream.Close()
	}
	if err == nil && c.mpQuic != nil {
		// Note closing the session here is fine since we know that all the traffic went through.
		// If you are not sure that this is the case you should probably not close the session.
		// E.g. if you are just sending something to a server and closing the session immediately
		// it might be that the server does not see the message.
		// See also: https://github.com/lucas-clemente/quic-go/issues/464
		err = c.mpQuic.Close(err)
	}
	if c.mpQuic != nil {
		err = c.mpQuic.CloseConn()
	}
	return err
}

func (c client) setupPaths() {
	if !remote.IA.Equal(local.IA) {
		pathEntries := choosePaths(*interactive)
		if pathEntries == nil {
			LogFatal("No paths available to remote destination")
		}
		for _, pathEntry := range pathEntries {
			newRemote := remote.Copy()
			newRemote.Path = spath.New(pathEntry.Path.FwdPath)
			newRemote.Path.InitOffsets()
			newRemote.NextHop, _ = pathEntry.HostInfo.Overlay()
			remotes = append(remotes, newRemote)
		}
	}
}

func imax(a, b int) (maximum int) {
	if a > b {
		maximum = a
	} else {
		maximum = b
	}
	return
}

func (c client) send() {
	for i := 0; i < *count || *count == 0; i++ {
		if i != 0 && *interval != 0 {
			time.Sleep(*interval)
		}

		reqMsg := requestMsg()

		// Send different payload size to correlate iterations in network capture
		infoString := fmt.Sprintf("This is the %vth message sent on this stream", i)
		fileData = []byte(infoString + strings.Repeat("A", imax(1000-len(infoString)-(100*(9-i)), len(infoString))))

		reqMsg = &message{
			BoingBoing: ReqMsg,
			Data:     fileData,
		}

		// Send boing? message to destination
		before := time.Now()
		reqMsg.Timestamp = before.UnixNano()

		err := c.WriteMsg(reqMsg)
		if err != nil {
			log.Error("Unable to write", "err", err)
			continue
		}
	}
	// After sending the last boing?, set a ReadDeadline on the stream
	err := c.qstream.SetReadDeadline(time.Now().Add(*timeout))
	if err != nil {
		LogFatal("SetReadDeadline failed", "err", err)
	}
}

func (c client) read() {
	// Receive boing! message (with final timeout)
	for i := 0; i < *count || *count == 0; i++ {
		msg, err := c.ReadMsg()
		after := time.Now()
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				log.Debug("ReadDeadline missed", "err", err)
				// ReadDeadline is only set after we are done writing
				// and we don't want to wait indefinitely for the remaining responses
				break
			}
			log.Error("Unable to read", "err", err)
			continue
		}
		if msg.BoingBoing != ReplyMsg {
			log.Error("Received wrong boingboing", "expected", ReplyMsg, "actual", msg.BoingBoing)
		}
		if !bytes.Equal(msg.Data, fileData) {
			log.Error("Received different data than sent.")
			continue
		}
		before := time.Unix(0, int64(msg.Timestamp))
		elapsed := after.Sub(before).Round(time.Microsecond)
		if *verbose {
			fmt.Printf("[%s]\tReceived %d bytes from %v: seq=%d RTT=%s\n",
				before.Format(common.TimeFmt), msg.len(), &remote, i, elapsed)
		} else {
			fmt.Printf("Received %d bytes from %v: seq=%d RTT=%s\n",
				msg.len(), &remote, i, elapsed)
		}
	}
}

type server struct {
}

// run listens on a SCION address and replies to any boing? message.
// On any error, the server exits.
func (s server) run() {
	// Listen on SCION address
	qsock, err := mpsquic.ListenSCION(nil, &local, nil)
	if err != nil {
		LogFatal("Unable to listen", "err", err)
	}
	log.Info("Listening", "local", qsock.Addr())
	for {
		qsess, err := qsock.Accept()
		if err != nil {
			log.Error("Unable to accept quic session", "err", err)
			// Accept failing means the socket is unusable.
			break
		}
		log.Info("Quic session accepted", "src", qsess.RemoteAddr())
		go func() {
			defer log.LogPanicAndExit()
			s.handleClient(qsess)
		}()
	}
}

func (s server) handleClient(qsess quic.Session) {
	var err error
	defer qsess.Close(err)
	qstream, err := qsess.AcceptStream()
	if err != nil {
		log.Error("Unable to accept quic stream", "err", err)
		return
	}
	defer qstream.Close()

	qs := newMPQuicStream(qstream)
	for {
		// Receive boing? message
		msg, err := qs.ReadMsg()
		if err != nil {
			log.Error("Unable to read", "err", err)
			break
		}

		// Send boing! message
		replyMsg := replyMsg(msg)
		err = qs.WriteMsg(replyMsg)
		if err != nil {
			log.Error("Unable to write", "err", err)
			break
		}
	}
}

func parsePathIndex(index string, max int) (pathIndex uint64, err error) {
	pathIndex, err = strconv.ParseUint(index, 10, 64)
	if err != nil {
		return 0, errors.New(fmt.Sprintf("Invalid choice: '%v', %v", index, err))
	}
	if int(pathIndex) > max {
		return 0, errors.New(fmt.Sprintf("Invalid choice: '%v', valid indices range: [0, %v]", index, max))
	}
	return
}

func parsePathChoice(selection string, max int) (pathIndices []uint64, err error) {
	var pathIndex uint64

	// Split tokens
	pathIndicesStr := strings.Split(selection[:len(selection)-1], " ")
	for _, pathIndexStr := range pathIndicesStr {
		if strings.Contains(pathIndexStr, "-") {
			// Handle ranges
			pathIndixRangeBoundaries := strings.Split(pathIndexStr, "-")
			if len(pathIndixRangeBoundaries) != 2 ||
				pathIndixRangeBoundaries[0] == "" ||
				pathIndixRangeBoundaries[1] == "" {
				return nil, errors.New(fmt.Sprintf("Invalid path range choice: '%v'", pathIndexStr))
			}

			pathIndexRangeStart, err := parsePathIndex(pathIndixRangeBoundaries[0], max)
			if err != nil {
				return nil, err
			}
			pathIndexRangeEnd, err := parsePathIndex(pathIndixRangeBoundaries[1], max)
			if err != nil {
				return nil, err
			}

			for i := pathIndexRangeStart; i <= pathIndexRangeEnd; i++ {
				pathIndices = append(pathIndices, i)
			}
		} else {
			// Handle individual entries
			pathIndex, err = parsePathIndex(pathIndexStr, max)
			if err != nil {
				return nil, err
			}
			pathIndices = append(pathIndices, pathIndex)
		}
	}
	if len(pathIndices) < 1 {
		return nil, errors.New(fmt.Sprintf("No path selected: '%v'", selection))
	}
	return pathIndices, nil
}

func choosePaths(interactive bool) []*sd.PathReplyEntry {
	var paths []*sd.PathReplyEntry
	var selectedPaths []*sd.PathReplyEntry

	pathMgr := snet.DefNetwork.PathResolver()
	pathSet := pathMgr.Query(context.Background(), local.IA, remote.IA, sd.PathReqFlags{})

	for _, p := range pathSet {
		paths = append(paths, p.Entry)
	}
	if  len(pathSet) == 0 || paths == nil {
		return nil
	}

	var pathIndices []uint64
	if interactive {
		fmt.Printf("Available paths to %v:\t(you can select multiple paths, such as ranges like A-C and multiple space separated path like B D F-H)\n", remote.IA)
		for i := range paths {
			fmt.Printf("[%2d] %s\n", i, paths[i].Path.String())
		}
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Printf("\nChoose paths: ")
			pathIndexStr, err := reader.ReadString('\n')
			pathIndices, err = parsePathChoice(pathIndexStr, len(paths)-1)
			if err != nil {
				_, err := fmt.Fprintf(os.Stderr,
					"ERROR: Invalid path selection. %v\n", err)
				if err != nil {
					log.Error("Unable to write to Stderr", "err", err)
				}
				continue
			}
			break
		}
	} else {
		pathIndices = append(pathIndices, 0)
	}

	for i := range pathIndices {
		selectedPaths = append(selectedPaths, paths[i])
	}

	fmt.Println("Using paths:")
	for _, path := range selectedPaths {
		fmt.Printf("  %s\n", path.Path.String())
	}
	return selectedPaths
}

func setSignalHandler(closer io.Closer) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer log.LogPanicAndExit()
		<-c
		closer.Close()
		os.Exit(1)
	}()
}
