package bwtestlib

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"sync"
	"time"

	log "github.com/inconshreveable/log15"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

const (
	// Maximum duration of a bandwidth test
	MaxDuration time.Duration = time.Second * 10
	// Maximum amount of time to wait for straggler packets
	StragglerWaitPeriod time.Duration = time.Second
	// Allow sending beyond the finish time by this amount
	GracePeriodSend time.Duration = time.Millisecond * 10
	// Min packet size is 4 bytes, so that 32-bit integer fits in
	// Ideally packet size > 4 bytes, so that part of the PRG is also in packet
	MinPacketSize int64 = 4
	// Max packet size to avoid allocation of too large a buffer, make it large enough for jumbo frames++
	MaxPacketSize int64 = 66000
	// Make sure the port number is a port the server application can connect to
	MinPort uint16 = 1024

	MaxTries int64         = 5 // Number of times to try to reach server
	Timeout  time.Duration = time.Millisecond * 500
	MaxRTT   time.Duration = time.Millisecond * 1000
)

type BwtestParameters struct {
	BwtestDuration time.Duration
	PacketSize     int64
	NumPackets     int64
	PrgKey         []byte
	Port           uint16
}

type BwtestResult struct {
	NumPacketsReceived int64
	CorrectlyReceived  int64
	IPAvar             int64
	IPAmin             int64
	IPAavg             int64
	IPAmax             int64
	// Contains the client's sending PRG key, so that the result can be uniquely identified
	// Only requests that contain the correct key can obtain the result
	PrgKey             []byte
	ExpectedFinishTime time.Time
}

func Check(e error) {
	if e != nil {
		LogFatal("Fatal error. Exiting.", "err", e)
	}
}

func LogFatal(msg string, a ...interface{}) {
	log.Crit(msg, a...)
	os.Exit(1)
}

// TODO: make it more generic: func LogPanicAndRestart(f func(a ...interface{}), a ...interface{}) {
func LogPanicAndRestart(f func(a *snet.Conn, b string, c []byte, d []byte), CCConn *snet.Conn, serverISDASIP string, receivePacketBuffer []byte, sendPacketBuffer []byte) {
	if msg := recover(); msg != nil {
		log.Crit("Panic", "msg", msg, "stack", string(debug.Stack()))
		log.Debug("Recovering from panic.")
		f(CCConn, serverISDASIP, receivePacketBuffer, sendPacketBuffer)
	}
}

// Fill buffer with AES PRG in counter mode
// The value of the ith 16-byte block is simply an encryption of i under the key
func PrgFill(key []byte, iv int, data []byte) {
	i := uint32(iv)
	aesCipher, err := aes.NewCipher(key)
	Check(err)
	pt := make([]byte, aes.BlockSize)
	j := 0
	for j <= len(data)-aes.BlockSize {
		binary.LittleEndian.PutUint32(pt, i)
		aesCipher.Encrypt(data, pt)
		j = j + aes.BlockSize
		i = i + uint32(aes.BlockSize)
	}
	// Check if fewer than BlockSize bytes are required for the final block
	if j < len(data) {
		binary.LittleEndian.PutUint32(pt, i)
		aesCipher.Encrypt(pt, pt)
		copy(data[j:], pt[:len(data)-j])
	}
}

// Encode BwtestResult into a sufficiently large byte buffer that is passed in, return the number of bytes written
func EncodeBwtestResult(res *BwtestResult, buf []byte) int {
	var bb bytes.Buffer
	enc := gob.NewEncoder(&bb)
	err := enc.Encode(*res)
	Check(err)
	copy(buf, bb.Bytes())
	return bb.Len()
}

// Decode BwtestResult from byte buffer that is passed in, returns BwtestResult structure and number of bytes consumed
func DecodeBwtestResult(buf []byte) (*BwtestResult, int, error) {
	bb := bytes.NewBuffer(buf)
	is := bb.Len()
	dec := gob.NewDecoder(bb)
	var v BwtestResult
	err := dec.Decode(&v)
	return &v, is - bb.Len(), err
}

// Encode BwtestParameters into a sufficiently large byte buffer that is passed in, return the number of bytes written
func EncodeBwtestParameters(bwtp *BwtestParameters, buf []byte) int {
	var bb bytes.Buffer
	enc := gob.NewEncoder(&bb)
	err := enc.Encode(*bwtp)
	Check(err)
	copy(buf, bb.Bytes())
	return bb.Len()
}

// Decode BwtestParameters from byte buffer that is passed in, returns BwtestParameters structure and number of bytes consumed
func DecodeBwtestParameters(buf []byte) (*BwtestParameters, int, error) {
	bb := bytes.NewBuffer(buf)
	is := bb.Len()
	dec := gob.NewDecoder(bb)
	var v BwtestParameters
	err := dec.Decode(&v)
	// Make sure that arguments are within correct parameter ranges
	if v.BwtestDuration > MaxDuration {
		v.BwtestDuration = MaxDuration
	}
	if v.BwtestDuration < time.Duration(0) {
		v.BwtestDuration = time.Duration(0)
	}
	if v.PacketSize < MinPacketSize {
		v.PacketSize = MinPacketSize
	}
	if v.PacketSize > MaxPacketSize {
		v.PacketSize = MaxPacketSize
	}
	if v.Port < MinPort {
		v.Port = MinPort
	}
	return &v, is - bb.Len(), err
}

func HandleDCConnSend(bwp *BwtestParameters, udpConnection *snet.Conn) {
	sb := make([]byte, bwp.PacketSize)
	var i int64 = 0
	t0 := time.Now()
	finish := t0.Add(bwp.BwtestDuration + GracePeriodSend)
	var interPktInterval time.Duration
	if bwp.NumPackets > 1 {
		interPktInterval = bwp.BwtestDuration / time.Duration(bwp.NumPackets-1)
	} else {
		interPktInterval = bwp.BwtestDuration
	}
	for i < bwp.NumPackets {
		// Compute how long to wait
		t1 := time.Now()
		if t1.After(finish) {
			// We've been sending for too long, sending bandwidth must be insufficient. Abort sending.
			return
		}
		t2 := t0.Add(interPktInterval * time.Duration(i))
		if t1.Before(t2) {
			time.Sleep(t2.Sub(t1))
		}
		// Send packet now
		PrgFill(bwp.PrgKey, int(i*bwp.PacketSize), sb)
		// Place packet number at the beginning of the packet, overwriting some PRG data
		binary.LittleEndian.PutUint32(sb, uint32(i*bwp.PacketSize))
		n, err := udpConnection.Write(sb)
		if err != nil {
			if common.GetErrorMsg(err) == "Path not found" { // TODO: add const error string to snet/conn and use that
				// Do not handle "Path not found" as fatal, log and skip
				log.Debug("No path to remote found", "err", common.FmtError(err))
			} else {
				Check(err)
			}
		} else if int64(n) < bwp.PacketSize {
			Check(fmt.Errorf("Insufficient number of bytes written:", n, "instead of:", bwp.PacketSize))
		}
		i++
	}
}

func HandleDCConnReceive(bwp *BwtestParameters, udpConnection *snet.Conn, res *BwtestResult, resLock *sync.Mutex, done *sync.Mutex) {
	resLock.Lock()
	finish := res.ExpectedFinishTime
	resLock.Unlock()
	var numPacketsReceived, correctlyReceived int64 = 0, 0
	InterPacketArrivalTime := make(map[int]int64)
	_ = udpConnection.SetReadDeadline(finish)
	// Make the receive buffer a bit larger to enable detection of packets that are too large
	recBuf := make([]byte, bwp.PacketSize+1000)
	cmpBuf := make([]byte, bwp.PacketSize)
	for time.Now().Before(finish) && correctlyReceived < bwp.NumPackets {
		n, err := udpConnection.Read(recBuf)
		// Ignore errors, todo: detect type of error and quit if it was because of a SetReadDeadline
		if err != nil {
			// If the ReadDeadline expired, then we should extend the finish time, which is
			// extended on the client side if no response is received from the server. On the server
			// side, however, a short BwtestDuration with several consecutive packet losses would
			// lead to closing the connection.
			resLock.Lock()
			finish = res.ExpectedFinishTime
			resLock.Unlock()
			continue
		}
		numPacketsReceived++
		if int64(n) != bwp.PacketSize {
			// The packet has incorrect size, do not count as a correct packet
			// fmt.Println("Incorrect size.", n, "bytes instead of", bwp.PacketSize)
			continue
		}
		// Could consider pre-computing all the packets in a separate goroutine
		// but since computation is usually much higher than bandwidth, this is
		// not necessary
		// Todo: create separate verif function which only compares the packet
		// so that a discrepancy is noticed immediately without generating the
		// entire packet
		iv := int64(binary.LittleEndian.Uint32(recBuf))
		seqNo := int(iv / bwp.PacketSize)
		InterPacketArrivalTime[seqNo] = time.Now().UnixNano()
		PrgFill(bwp.PrgKey, int(iv), cmpBuf)
		binary.LittleEndian.PutUint32(cmpBuf, uint32(iv))
		if bytes.Equal(recBuf[:bwp.PacketSize], cmpBuf) {
			if correctlyReceived == 0 {
				// Adjust finish time after first correctly received packet
				// Note that we should check that we're not too far away from the beginning of the
				// bwtest, otherwise we're extending the time for too long. If the server's 'N' response
				// packet was not dropped, then sending should start within MaxRTT at most.
				newFinish := time.Now().Add(bwp.BwtestDuration + StragglerWaitPeriod)
				if newFinish.After(finish) {
					finish = newFinish
					_ = udpConnection.SetReadDeadline(finish)
					resLock.Lock()
					if res.ExpectedFinishTime.Before(finish) {
						// Most likely what happened is that the server's 'N' response packet got dropped (in case this
						// is the receive function on the server side) or the client's request packet got dropped (in
						// case this is the receive function on the client side). In both cases the ExpectedFinishTime
						// needs to be updated
						res.ExpectedFinishTime = finish
					}
					resLock.Unlock()
				}
			}
			correctlyReceived++
		}
	}

	resLock.Lock()
	res.NumPacketsReceived = numPacketsReceived
	res.CorrectlyReceived = correctlyReceived
	res.IPAvar, res.IPAmin, res.IPAavg, res.IPAmax = aggrInterArrivalTime(InterPacketArrivalTime)

	// We're done here, let's see if we need to wait for the send function to complete so we can close the connection
	// Note: the locking here is not strictly necessary, since ExpectedFinishTime is only updated right after
	// initialization and in the code above, but it's good practice to do always lock when using the variable
	eft := res.ExpectedFinishTime
	resLock.Unlock()
	if done != nil {
		// Signal that we're done
		done.Unlock()
	}
	if time.Now().Before(eft) {
		time.Sleep(eft.Sub(time.Now()))
	}
	_ = udpConnection.Close()
}

func aggrInterArrivalTime(bwr map[int]int64) (IPAvar, IPAmin, IPAavg, IPAmax int64) {
	// reverse map, mapping timestamps to sequence numbers
	revMap := make(map[int64]int)
	var keys []int64 // keys are the timestamps of the received packets
	// fill the reverse map and the keep track of the timestamps
	for k, v := range bwr {
		revMap[v] = k
		keys = append(keys, v)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] }) // sorted timestamps of the received packets

	// We keep only the interarrival times of successive packets with no drops
	var iat []int64
	i := 1
	for i < len(keys) {
		if revMap[keys[i-1]]+1 == revMap[keys[i]] { // valid measurement without reordering, include
			iat = append(iat, keys[i]-keys[i-1]) // resulting interarrival time
		}
		i += 1
	}

	// Compute variance and average
	var average float64 = 0
	IPAmin = -1
	for _, v := range iat {
		if v > IPAmax {
			IPAmax = v
		}
		if v < IPAmin || IPAmin == -1 {
			IPAmin = v
		}
		average += float64(v) / float64(len(iat))
	}
	IPAvar = IPAmax - int64(average)
	IPAavg = int64(average)
	return
}

func ChoosePath(interactive bool, pathAlgo string, local snet.Addr, remote snet.Addr) *sciond.PathReplyEntry {
	pathMgr := snet.DefNetwork.PathResolver()
	pathSet := pathMgr.Query(local.IA, remote.IA)
	var appPaths []*spathmeta.AppPath
	var selectedPath *spathmeta.AppPath

	if len(pathSet) == 0 {
		return nil
	}

	fmt.Printf("Available paths to %v\n", remote.IA)
	i := 0
	for _, path := range pathSet {
		appPaths = append(appPaths, path)
		fmt.Printf("[%2d] %s\n", i, path.Entry.Path.String())
		i++
	}

	if interactive {
		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Printf("Choose path: ")
			scanner.Scan()
			pathIndexStr := scanner.Text()
			pathIndex, err := strconv.Atoi(pathIndexStr)
			if err == nil && 0 <= pathIndex && pathIndex < len(appPaths) {
				selectedPath = appPaths[pathIndex]
				break
			}
			fmt.Printf("ERROR: Invalid path index %v, valid indices range: [0, %v]\n", pathIndex, len(appPaths)-1)
		}
	} else {
		// when in non-interactive mode, use path selection function to choose path
		selectedPath = pathSelection(pathSet, pathAlgo)
	}
	entry := selectedPath.Entry
	fmt.Printf("Using path:\n  %s\n", entry.Path.String())
	return entry
}

func pathSelection(pathSet spathmeta.AppPathSet, pathAlgo string) *spathmeta.AppPath {
	var selectedPath *spathmeta.AppPath
	var metric float64
	// A path selection algorithm consists of a simple comparision function selecting the best path according
	// to some path property and a metric function normalizing that property to a value in [0,1], where larger is better
	// Available path selection algorithms, the metric returned must be normalized between [0,1]:
	pathAlgos := map[string](func(spathmeta.AppPathSet) (*spathmeta.AppPath, float64)){
		"shortest": selectShortestPath,
		"mtu":      selectLargestMTUPath,
	}
	switch pathAlgo {
	case "shortest":
		log.Debug("Path selection algorithm", "pathAlgo", "shortest")
		selectedPath, metric = pathAlgos[pathAlgo](pathSet)
	case "mtu":
		log.Debug("Path selection algorithm", "pathAlgo", "MTU")
		selectedPath, metric = pathAlgos[pathAlgo](pathSet)
	default:
		// Default is to take result with best score
		for _, algo := range pathAlgos {
			cadidatePath, cadidateMetric := algo(pathSet)
			if cadidateMetric > metric {
				selectedPath = cadidatePath
				metric = cadidateMetric
			}
		}
	}
	log.Debug("Path selection algorithm choice", "path", selectedPath.Entry.Path.String(), "score", metric)
	return selectedPath
}

func selectShortestPath(pathSet spathmeta.AppPathSet) (selectedPath *spathmeta.AppPath, metric float64) {
	// Selects shortest path by number of hops
	for _, appPath := range pathSet {
		if selectedPath == nil || len(appPath.Entry.Path.Interfaces) < len(selectedPath.Entry.Path.Interfaces) {
			selectedPath = appPath
		}
	}
	metric_fn := func(rawMetric []sciond.PathInterface) (result float64) {
		hopCount := float64(len(rawMetric))
		midpoint := 7.0
		result = math.Exp(-(hopCount - midpoint)) / (1 + math.Exp(-(hopCount - midpoint)))
		return result
	}
	return selectedPath, metric_fn(selectedPath.Entry.Path.Interfaces)
}

func selectLargestMTUPath(pathSet spathmeta.AppPathSet) (selectedPath *spathmeta.AppPath, metric float64) {
	// Selects path with largest MTU
	for _, appPath := range pathSet {
		if selectedPath == nil || appPath.Entry.Path.Mtu > selectedPath.Entry.Path.Mtu {
			selectedPath = appPath
		}
	}
	metric_fn := func(rawMetric uint16) (result float64) {
		mtu := float64(rawMetric)
		midpoint := 1500.0
		tilt := 0.004
		result = 1 / (1 + math.Exp(-tilt*(mtu-midpoint)))
		return result
	}
	return selectedPath, metric_fn(selectedPath.Entry.Path.Mtu)
}
