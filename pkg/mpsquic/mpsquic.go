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

// "Multiple paths" QUIC/SCION implementation.
package mpsquic

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/lucas-clemente/quic-go"
	//"github.com/lucas-clemente/quic-go/quictrace"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/hostinfo"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/overlay"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/scmp"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

const (
	defScionConfPath = "/etc/scion/"
	defKeyPath       = "gen-certs/tls.key"
	defPemPath       = "gen-certs/tls.pem"
)

const (
	maxDuration time.Duration = 1<<63 - 1
)

var _ quic.Session = (*MPQuic)(nil)

type pathInfo struct {
	raddr      *snet.Addr
	path       spathmeta.AppPath
	appPathKey spathmeta.PathKey // caches path.Key()
	rawPathKey RawKey            // caches
	expiration time.Time
	rtt        time.Duration
	bw         int // in bps
}

type MPQuic struct {
	quic.Session
	scionFlexConnection *SCIONFlexConn
	network             *snet.SCIONNetwork
	dispConn            *reliable.Conn
	paths               []pathInfo
	active              *pathInfo
}

type Logger struct {
	Trace func(msg string, ctx ...interface{})
	Debug func(msg string, ctx ...interface{})
	Info  func(msg string, ctx ...interface{})
	Warn  func(msg string, ctx ...interface{})
	Error func(msg string, ctx ...interface{})
	Crit  func(msg string, ctx ...interface{})
}

type keyedRevocation struct {
	key            *RawKey
	revocationInfo *scmp.InfoRevocation
}

var (
	logger *Logger

	revocationQ chan keyedRevocation

	//tracer quictrace.Tracer
	// Don't verify the server's cert, as we are not using the TLS PKI.
	cliTlsCfg = &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"SCION"},
	}
	srvTlsCfg = &tls.Config{NextProtos: []string{"SCION"}}
)

// Init initializes the SCION networking context and the QUIC session's crypto.
func Init(ia addr.IA, sciondPath, dispatcher, keyPath, pemPath string) error {
	if logger == nil {
		// By default this library is noisy, to mute it call msquic.MuteLogging
		initLogging(log.Root())
	}
	/*
		// Default SCION networking context without custom SCMP handler
		if err := snet.Init(ia, sciondPath, reliable.NewDispatcherService(dispatcher)); err != nil {
			return common.NewBasicError("mpsquic: Unable to initialize SCION network", err)
		}
	*/
	if revocationQ == nil {
		revocationQ = make(chan keyedRevocation, 50) // Buffered channel, we can buffer up to 1 revocation per 20ms for 1s.
	}
	if err := initNetworkWithPRCustomSCMPHandler(ia, sciondPath, reliable.NewDispatcherService(dispatcher)); err != nil {
		return common.NewBasicError("mpsquic: Unable to initialize SCION network", err)
	}
	if keyPath == "" {
		keyPath = defScionConfPath + defKeyPath
	}
	if pemPath == "" {
		pemPath = defScionConfPath + defPemPath
	}
	cert, err := tls.LoadX509KeyPair(pemPath, keyPath)
	if err != nil {
		return common.NewBasicError("mpsquic: Unable to load TLS cert/key", err)
	}
	srvTlsCfg.Certificates = []tls.Certificate{cert}
	return nil
}

// initLogging initializes logging for the mpsquic library using the passed scionproto (or similar) logger
func initLogging(baseLogger log.Logger) {
	logger = &Logger{}
	logger.Trace = func(msg string, ctx ...interface{}) { baseLogger.Trace("MSQUIC: "+msg, ctx...) }
	logger.Debug = func(msg string, ctx ...interface{}) { baseLogger.Debug("MSQUIC: "+msg, ctx...) }
	logger.Info = func(msg string, ctx ...interface{}) { baseLogger.Info("MSQUIC: "+msg, ctx...) }
	logger.Warn = func(msg string, ctx ...interface{}) { baseLogger.Warn("MSQUIC: "+msg, ctx...) }
	logger.Error = func(msg string, ctx ...interface{}) { baseLogger.Error("MSQUIC: "+msg, ctx...) }
	logger.Crit = func(msg string, ctx ...interface{}) { baseLogger.Crit("MSQUIC: "+msg, ctx...) }
}

// SetBasicLogging sets mpsquic logging to only write to os.Stdout and os.Stderr
func SetBasicLogging() {
	if logger != nil {
		logger.Trace = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stdout, "%v\t%v", msg, ctx) }
		logger.Debug = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stdout, "%v\t%v", msg, ctx) }
		logger.Info = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stdout, "%v\t%v", msg, ctx) }
		logger.Warn = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stdout, "%v\t%v", msg, ctx) }
		logger.Error = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stderr, "%v\t%v", msg, ctx) }
		logger.Crit = func(msg string, ctx ...interface{}) { _, _ = fmt.Fprintf(os.Stderr, "%v\t%v", msg, ctx) }
	}
}

// MuteLogging mutes all logging in this library
func MuteLogging() {
	if logger != nil {
		logger.Trace = func(msg string, ctx ...interface{}) {}
		logger.Debug = func(msg string, ctx ...interface{}) {}
		logger.Info = func(msg string, ctx ...interface{}) {}
		logger.Warn = func(msg string, ctx ...interface{}) {}
		logger.Error = func(msg string, ctx ...interface{}) {}
		logger.Crit = func(msg string, ctx ...interface{}) {}
	}
}

// OpenStreamSync opens a QUIC stream over the QUIC session.
// It returns a QUIC stream ready to be written/read.
func (mpq *MPQuic) OpenStreamSync(ctx context.Context) (quic.Stream, error) {
	stream, err := mpq.Session.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return monitoredStream{stream, mpq}, nil
}

// Close closes the QUIC session.
func (mpq *MPQuic) Close() error {
	if err := exportTraces(); err != nil {
		logger.Warn("Failed to export QUIC trace", "err", err)
	}
	if mpq.Session != nil {
		return mpq.Session.Close()
	}
	return nil
}

// CloseConn closes the embedded SCION connection.
func (mpq *MPQuic) CloseConn() error {
	if mpq.dispConn != nil {
		tmp := mpq.dispConn
		mpq.dispConn = nil
		time.Sleep(time.Second)
		err := tmp.Close()
		if err != nil {
			return err
		}
	}
	return mpq.scionFlexConnection.Close()
}

// DialMP creates a monitored multiple paths connection using QUIC over SCION.
// It returns a MPQuic struct if opening a QUIC session over the initial SCION path succeeded.
func DialMP(network *snet.SCIONNetwork, laddr *snet.Addr, raddr *snet.Addr, paths []spathmeta.AppPath,
	quicConfig *quic.Config) (*MPQuic, error) {

	return DialMPWithBindSVC(network, laddr, raddr, paths, nil, addr.SvcNone, quicConfig)
}

// createSCMPMonitorConn opens a connection to the default dispatcher.
// It returns a reliable socket connection.
func createSCMPMonitorConn(laddr, baddr *snet.Addr) (dispConn *reliable.Conn, err error) {
	// Connect to the dispatcher
	var overlayBindAddr *overlay.OverlayAddr
	if baddr != nil {
		if baddr.Host != nil {
			overlayBindAddr, err = overlay.NewOverlayAddr(baddr.Host.L3, baddr.Host.L4)
			if err != nil {
				logger.Error("Failed to create bind address", "err", err)
				return nil, err
			}
		}
	}
	laddrMonitor := laddr.Copy()
	laddrMonitor.Host.L4 = addr.NewL4UDPInfo(0) // Use any free port
	dispConn, _, err = reliable.Register(reliable.DefaultDispPath, laddrMonitor.IA, laddrMonitor.Host,
		overlayBindAddr, addr.SvcNone)
	if err != nil {
		logger.Error("Unable to register with the dispatcher", "addr", laddrMonitor, "err", err)
		return nil, err
	}
	return dispConn, nil
}

// Creates an AppPath using the spath.Path and hostinfo addr.AppAddr available on a snet.Addr, missing values are set to their zero value.
func mockAppPath(spathP *spath.Path, host *addr.AppAddr) (appPath *spathmeta.AppPath, err error) {
	appPath = &spathmeta.AppPath{
		Entry: &sciond.PathReplyEntry{
			Path:     nil,
			HostInfo: *hostinfo.FromHostAddr(addr.HostFromIPStr("127.0.0.1"), 30041)}}
	if host != nil {
		appPath.Entry.HostInfo = *hostinfo.FromHostAddr(host.L3, host.L4.Port())
	}
	if spathP == nil {
		return appPath, nil
	}

	cpath, err := parseSPath(*spathP)
	if err != nil {
		return nil, err
	}

	appPath.Entry.Path = &sciond.FwdPathMeta{
		FwdPath:    spathP.Raw,
		Mtu:        cpath.Mtu,
		Interfaces: cpath.Interfaces,
		ExpTime:    uint32(cpath.ComputeExpTime().Unix())}
	return appPath, nil
}

// DialMPWithBindSVC creates a monitored multiple paths connection using QUIC over SCION on the specified bind address baddr.
// It returns a MPQuic struct if a opening a QUIC session over the initial SCION path succeeded.
func DialMPWithBindSVC(network *snet.SCIONNetwork, laddr *snet.Addr, raddr *snet.Addr, paths []spathmeta.AppPath, baddr *snet.Addr,
	svc addr.HostSVC, quicConfig *quic.Config) (*MPQuic, error) {

	if network == nil {
		network = snet.DefNetwork
	}

	sconn, err := sListen(network, laddr, baddr, svc)
	if err != nil {
		return nil, err
	}

	dispConn, err := createSCMPMonitorConn(laddr, baddr)
	if err != nil {
		return nil, err
	}

	if paths == nil {
		paths = []spathmeta.AppPath{}
		// Infer path meta information from path on raddr, since no paths were provided
		appPath, err := mockAppPath(raddr.Path, raddr.Host)
		if err != nil {
			return nil, err
		}
		paths = append(paths, *appPath)
	}

	var raddrs []*snet.Addr = []*snet.Addr{}
	// Initialize a raddr for each path
	for i, p := range paths {
		logger.Info("Path", "index", i, "interfaces", p.Entry.Path.Interfaces)
		r := raddr.Copy()
		if p.Entry.Path != nil {
			r.Path = spath.New(p.Entry.Path.FwdPath)
		}
		if r.Path != nil {
			_ = r.Path.InitOffsets()
		}
		r.NextHop, _ = p.Entry.HostInfo.Overlay()
		raddrs = append(raddrs, r)
	}

	pathInfos := []pathInfo{}
	for i, raddr := range raddrs {
		spathRepr := spath.New(paths[i].Entry.Path.FwdPath)
		rawSpathKey, err := getSpathKey(*spathRepr)
		if err != nil {
			rspk := RawKey([]byte{})
			rawSpathKey = &rspk
		}
		pi := pathInfo{
			raddr:      raddr,
			path:       paths[i],
			appPathKey: paths[i].Key(),
			rawPathKey: *rawSpathKey,
			expiration: time.Time{},
			rtt:        maxDuration,
			bw:         0,
		}
		pathInfos = append(pathInfos, pi)
	}

	mpQuic, err := newMPQuic(sconn, laddr, network, quicConfig, dispConn, pathInfos)
	if err != nil {
		return nil, err
	}
	mpQuic.monitor()

	return mpQuic, nil
}

func newMPQuic(sconn snet.Conn, laddr *snet.Addr, network *snet.SCIONNetwork, quicConfig *quic.Config, dispConn *reliable.Conn, pathInfos []pathInfo) (mpQuic *MPQuic, err error) {
	active := &pathInfos[0]
	mpQuic = &MPQuic{Session: nil, scionFlexConnection: nil, network: network, dispConn: dispConn, paths: pathInfos, active: active}
	logger.Info("Active AppPath", "key", active.appPathKey, "Hops", active.path.Entry.Path.Interfaces)
	flexConn := newSCIONFlexConn(sconn, mpQuic, laddr, active.raddr)
	mpQuic.scionFlexConnection = flexConn

	/*if quicConfig != nil {
		tracer = quicConfig.QuicTracer
	}*/

	// Use dummy hostname, as it's used for SNI, and we're not doing cert verification.
	qsession, err := quic.Dial(flexConn, flexConn.raddr, "host:0", cliTlsCfg, quicConfig)
	if err != nil {
		return nil, err
	}
	mpQuic.Session = qsession
	return mpQuic, nil
}

// displayStats logs the collected metrics for all monitored paths.
func (mpq *MPQuic) displayStats() {
	for i, pathInfo := range mpq.paths {
		logger.Debug(fmt.Sprintf("Path %v will expire at %v.\n", i, pathInfo.expiration))
		logger.Debug(fmt.Sprintf("Measured RTT of %v on path %v.\n", pathInfo.rtt, i))
		logger.Debug(fmt.Sprintf("Measured approximate BW of %v Mbps on path %v.\n", pathInfo.bw/1e6, i))
	}
}

// policyLowerRTTMatch returns true if the path with candidate index has a lower RTT than the active path.
func (mpq *MPQuic) policyLowerRTTMatch(candidate int) bool {
	return mpq.paths[candidate].rtt < mpq.active.rtt
}

// updateActivePath updates the active path in a thread safe manner.
func (mpq *MPQuic) updateActivePath(newPathIndex int) {
	// Lock the connection raddr, and update both the active path and the raddr of the FlexConn.
	mpq.scionFlexConnection.addrMtx.Lock()
	defer mpq.scionFlexConnection.addrMtx.Unlock()
	mpq.active = &mpq.paths[newPathIndex]
	mpq.scionFlexConnection.setRemoteAddr(mpq.active.raddr)
}

// switchMPConn switches between different SCION paths as given by the SCION address with path structs in paths.
// The force flag makes switching a requirement, set it when continuing to use the existing path is not an option.
func (mpq *MPQuic) switchMPConn(force bool, filter bool) error {
	mpq.displayStats()
	if force {
		// Always refresh available paths, as failing to find a fresh path leads to a hard failure
		mpq.refreshPaths(mpq.network.PathResolver())
	}
	for i := range mpq.paths {
		// Do not switch to identical path or to expired path
		if mpq.scionFlexConnection.raddr != mpq.paths[i].raddr && mpq.paths[i].expiration.After(time.Now()) {
			logger.Trace("Previous path", "path", mpq.scionFlexConnection.raddr.Path)
			logger.Trace("New path", "path", mpq.paths[i].raddr.Path)
			if !filter {
				mpq.updateActivePath(i)
				logger.Debug("Updating to path", "index", i, "path", mpq.paths[i].path.Entry.Path.Interfaces)
				return nil
			}
			if mpq.policyLowerRTTMatch(i) {
				mpq.updateActivePath(i)
				logger.Debug("Updating to better path", "index", i, "path", mpq.paths[i].path.Entry.Path.Interfaces)
				return nil
			}
		}
	}
	if !force {
		return nil
	}
	logger.Debug("No path available now", "now", time.Now())
	mpq.displayStats()

	return common.NewBasicError("mpsquic: No fallback connection available.", nil)
}
