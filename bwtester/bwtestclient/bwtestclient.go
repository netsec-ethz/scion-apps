// bwtestclient application
// For more documentation on the application see:
// https://github.com/netsec-ethz/scion-apps/blob/master/README.md
// https://github.com/netsec-ethz/scion-apps/blob/master/bwtester/README.md
package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	. "github.com/netsec-ethz/scion-apps/bwtester/bwtestlib"
	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
)

const (
	DefaultBwtestParameters = "3,1000,30,80kbps"
	DefaultDuration         = 3
	DefaultPktSize          = 1000
	DefaultPktCount         = 30
	DefaultBW               = 3000
	WildcardChar            = "?"
	MinBandwidth            = 5000 // 5 kbps is the minimum bandwidth we can test
)

var (
	InferedPktSize int64
	overlayType    string
)

func prepareAESKey() []byte {
	key := make([]byte, 16)
	n, err := rand.Read(key)
	Check(err)
	if n != 16 {
		Check(fmt.Errorf("Did not obtain 16 bytes of random information, only received %d", n))
	}
	return key
}

func printUsage() {
	fmt.Println("bwtestclient -c ClientSCIONAddress -s ServerSCIONAddress -cs t,size,num,bw -sc t,size,num,bw -i")
	fmt.Println("A SCION address is specified as ISD-AS,[IP Address]:Port")
	fmt.Println("Example SCION address 1-1011,[192.33.93.166]:42002")
	fmt.Println("ClientSCIONAddress can be omitted, the application then binds to localhost")
	fmt.Println("-cs specifies time duration (seconds), packet size (bytes), number of packets, target bandwidth " +
		"of client->server test")
	fmt.Println("\tThe question mark character ? can be used as wildcard when setting the test parameters " +
		"and its value is computed according to the other parameters. When more than one wilcard is used, " +
		"all but the last one are set to the default values, e.g. ?,1000,?,5Mbps will run the test for the " +
		"default duration and send as many packets as required to reach a bandwidth of 5 Mbps with the given " +
		"packet size.")
	fmt.Println("\tSupported bandwidth unit prefixes are: none (e.g. 1500bps for 1.5kbps), k, M, G, T.")
	fmt.Println("\tYou can also only set the target bandwidth, e.g. -cs 1Mbps")
	fmt.Println("-sc specifies time duration, packet size, number of packets, target bandwidth of server->client " +
		"test")
	fmt.Println("\tYou can also only set the target bandwidth, e.g. -sc 1500kbps")
	fmt.Println("\tWhen only the cs or sc flag is set, the other flag is set to the same value.")
	fmt.Println("-i specifies if the client is used in interactive mode, " +
		"when true the user is prompted for a path choice")
	fmt.Println("Default test parameters are: ", DefaultBwtestParameters)
}

// Input format (time duration,packet size,number of packets,target bandwidth), no spaces, question mark ? is wildcard
// The value of the wildcard is computed from the other values, if more than one wildcard is used,
// all but the last one are set to the defaults values
func parseBwtestParameters(s string) BwtestParameters {
	if !strings.Contains(s, ",") {
		// Using simple bandwidth setting with all defaults except bandwidth
		s = "?,?,?," + s
	}
	a := strings.Split(s, ",")
	if len(a) != 4 {
		Check(fmt.Errorf("Incorrect number of arguments, need 4 values for bwtestparameters. "+
			"You can use ? as wildcard, e.g. %s", DefaultBwtestParameters))
	}
	wildcards := 0
	for _, v := range a {
		if v == WildcardChar {
			wildcards += 1
		}
	}

	var a1, a2, a3, a4 int64
	if a[0] == WildcardChar {
		wildcards -= 1
		if wildcards == 0 {
			a2 = getPacketSize(a[1])
			a3 = getPacketCount(a[2])
			a4 = parseBandwidth(a[3])
			a1 = (a2 * 8 * a3) / a4
			if time.Second*time.Duration(a1) > MaxDuration {
				fmt.Printf("Duration is exceeding MaxDuration: %v > %v, using default value %d\n",
					a1, MaxDuration/time.Second, DefaultDuration)
				fmt.Println("Target bandwidth might no be reachable with that parameter.")
				a1 = DefaultDuration
			}
			if a1 < 1 {
				fmt.Printf("Duration is too short: %v , using default value %d\n",
					a1, DefaultDuration)
				fmt.Println("Target bandwidth might no be reachable with that parameter.")
				a1 = DefaultDuration
			}
		} else {
			a1 = DefaultDuration
		}
	} else {
		a1 = getDuration(a[0])
	}
	if a[1] == WildcardChar {
		wildcards -= 1
		if wildcards == 0 {
			a3 = getPacketCount(a[2])
			a4 = parseBandwidth(a[3])
			a2 = (a4 * a1) / (a3 * 8)
		} else {
			a2 = InferedPktSize
		}
	} else {
		a2 = getPacketSize(a[1])
	}
	if a[2] == WildcardChar {
		wildcards -= 1
		if wildcards == 0 {
			a4 = parseBandwidth(a[3])
			a3 = (a4 * a1) / (a2 * 8)
		} else {
			a3 = DefaultPktCount
		}
	} else {
		a3 = getPacketCount(a[2])
	}
	if a[3] == WildcardChar {
		wildcards -= 1
		if wildcards == 0 {
			fmt.Printf("Target bandwidth is %d\n", a2*a3*8/a1)
		}
	} else {
		a4 = parseBandwidth(a[3])
		// allow a deviation of up to one packet per 1 second interval, since we do not send half-packets
		if a2*a3*8/a1 > a4+a2*a1 || a2*a3*8/a1 < a4-a2*a1 {
			Check(fmt.Errorf("Computed target bandwidth does not match parameters, "+
				"use wildcard or specify correct bandwidth, expected %d, provided %d",
				a2*a3*8/a1, a4))
		}
	}
	key := prepareAESKey()
	return BwtestParameters{time.Second * time.Duration(a1), a2, a3, key, 0}
}

func parseBandwidth(bw string) int64 {
	rawBw := strings.Split(bw, "bps")
	if len(rawBw[0]) < 1 {
		fmt.Printf("Invalid bandwidth %v provided, using default value %d\n", bw, DefaultBW)
		return DefaultBW
	}

	var m int64
	val := rawBw[0][:len(rawBw[0])-1]
	suffix := rawBw[0][len(rawBw[0])-1:]
	switch suffix {
	case "k":
		m = 1e3
		break
	case "M":
		m = 1e6
		break
	case "G":
		m = 1e9
		break
	case "T":
		m = 1e12
		break
	default:
		m = 1
		val = rawBw[0]
		// ensure that the string ends with a digit
		if !unicode.IsDigit(([]rune(suffix))[0]) {
			fmt.Printf("Invalid bandwidth %v provided, using default value %d\n", val, DefaultBW)
			return DefaultBW
		}
	}

	a4, err := strconv.ParseInt(val, 10, 64)
	if err != nil || a4 < 0 {
		fmt.Printf("Invalid bandwidth %v provided, using default value %d\n", val, DefaultBW)
		return DefaultBW
	}

	return a4 * m
}

func getDuration(duration string) int64 {
	a1, err := strconv.ParseInt(duration, 10, 64)
	if err != nil || a1 <= 0 {
		fmt.Printf("Invalid duration %v provided, using default value %d\n", a1, DefaultDuration)
		a1 = DefaultDuration
	}
	d := time.Second * time.Duration(a1)
	if d > MaxDuration {
		Check(fmt.Errorf("Duration is exceeding MaxDuration: %d > %d", a1, MaxDuration/time.Second))
		a1 = DefaultDuration
	}
	return a1
}

func getPacketSize(size string) int64 {
	a2, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		fmt.Printf("Invalid packet size %v provided, using default value %d\n", a2, InferedPktSize)
		a2 = InferedPktSize
	}

	if a2 < MinPacketSize {
		a2 = MinPacketSize
	}
	if a2 > MaxPacketSize {
		a2 = MaxPacketSize
	}
	return a2
}

func getPacketCount(count string) int64 {
	a3, err := strconv.ParseInt(count, 10, 64)
	if err != nil || a3 <= 0 {
		fmt.Printf("Invalid packet count %v provided, using default value %d\n", a3, DefaultPktCount)
		a3 = DefaultPktCount
	}
	return a3
}

func main() {
	var (
		sciondPath      string
		sciondFromIA    bool
		dispatcherPath  string
		clientCCAddrStr string
		serverCCAddrStr string
		clientPort      uint

		// Control channel connection
		CCConn snet.Conn
		// Address of client control channel (CC)
		clientCCAddr *snet.Addr
		// Address of server control channel (CC)
		serverCCAddr *snet.Addr

		// Address of client data channel (DC)
		clientDCAddr *snet.Addr
		// Address of server data channel (DC)
		serverDCAddr *snet.Addr
		// Data channel connection
		DCConn snet.Conn

		clientBwpStr string
		clientBwp    BwtestParameters
		serverBwpStr string
		serverBwp    BwtestParameters
		interactive  bool
		pathAlgo     string
		useIPv6      bool

		maxBandwidth bool

		err error
	)
	flag.StringVar(&sciondPath, "sciond", "", "Path to sciond socket")
	flag.BoolVar(&sciondFromIA, "sciondFromIA", false, "SCIOND socket path from IA address:ISD-AS")
	flag.StringVar(&dispatcherPath, "dispatcher", "/run/shm/dispatcher/default.sock",
		"Path to dispatcher socket")
	flag.StringVar(&clientCCAddrStr, "c", "", "Client SCION Address")
	flag.UintVar(&clientPort, "p", 0, "Client Port (only used when Client Address not set)")
	flag.StringVar(&serverCCAddrStr, "s", "", "Server SCION Address")
	flag.StringVar(&serverBwpStr, "sc", DefaultBwtestParameters, "Server->Client test parameter")
	flag.StringVar(&clientBwpStr, "cs", DefaultBwtestParameters, "Client->Server test parameter")
	flag.BoolVar(&interactive, "i", false, "Interactive mode")
	flag.StringVar(&pathAlgo, "pathAlgo", "", "Path selection algorithm / metric (\"shortest\", \"mtu\")")
	flag.BoolVar(&useIPv6, "6", false, "Use IPv6")
	flag.BoolVar(&maxBandwidth, "findMax", false, "Find the maximum bandwidth achievable.\nYou can"+
		"use the flags \"cs\" and \"sc\" to set the parameters along with initial bandwidth to test on the link.\n"+
		"The other parameters will be fixed except for the packet count which will change in every run.\nThe higher"+
		" the duration of the test, the more accurate the results, but it will take longer to find the "+
		"maximum bandwidth.")

	flag.Parse()
	flagset := make(map[string]bool)
	// record if flags were set or if default value was used
	flag.Visit(func(f *flag.Flag) { flagset[f.Name] = true })

	if flag.NFlag() == 0 {
		// no flag was set, only print usage and exit
		printUsage()
		os.Exit(0)
	}

	if useIPv6 {
		overlayType = "udp6"
	} else {
		overlayType = "udp4"
	}
	// Create SCION UDP socket
	if len(clientCCAddrStr) == 0 {
		clientCCAddrStr, err = scionutil.GetLocalhostString()
		clientCCAddrStr = fmt.Sprintf("%s:%d", clientCCAddrStr, clientPort)
	}
	Check(err)

	clientCCAddr, err = snet.AddrFromString(clientCCAddrStr)
	Check(err)

	if len(serverCCAddrStr) > 0 {
		serverCCAddr, err = snet.AddrFromString(serverCCAddrStr)
		Check(err)
	} else {
		printUsage()
		Check(fmt.Errorf("Error, server address needs to be specified with -s"))
	}

	if sciondFromIA {
		if sciondPath != "" {
			LogFatal("Only one of -sciond or -sciondFromIA can be specified")
		}
		sciondPath = sciond.GetDefaultSCIONDPath(&clientCCAddr.IA)
	} else if sciondPath == "" {
		sciondPath = sciond.GetDefaultSCIONDPath(nil)
	}

	// initalize the addresses before passing them
	serverDCAddr, clientDCAddr = &snet.Addr{}, &snet.Addr{}
	CCConn, DCConn, err = initConns(serverCCAddr, serverDCAddr, clientCCAddr, clientDCAddr, pathAlgo, interactive)
	if err != nil {
		Check(err)
	}

	if !flagset["cs"] && flagset["sc"] { // Only one direction set, used same for reverse
		clientBwpStr = serverBwpStr
		fmt.Println("Only sc parameter set, using same values for cs")
	}
	clientBwp = parseBwtestParameters(clientBwpStr)
	clientBwp.Port = clientDCAddr.Host.L4.Port()
	if !flagset["sc"] && flagset["cs"] { // Only one direction set, used same for reverse
		serverBwpStr = clientBwpStr
		fmt.Println("Only cs parameter set, using same values for sc")
	}
	serverBwp = parseBwtestParameters(serverBwpStr)
	serverBwp.Port = serverDCAddr.Host.L4.Port()

	fmt.Println("\nTest parameters:")
	fmt.Println("clientDCAddr -> serverDCAddr", clientDCAddr, "->", serverDCAddr)
	fmt.Printf("client->server: %d seconds, %d bytes, %d packets\n",
		int(clientBwp.BwtestDuration/time.Second), clientBwp.PacketSize, clientBwp.NumPackets)
	fmt.Printf("server->client: %d seconds, %d bytes, %d packets\n",
		int(serverBwp.BwtestDuration/time.Second), serverBwp.PacketSize, serverBwp.NumPackets)

	if maxBandwidth {
		findMaxBandwidth(CCConn, DCConn, serverCCAddr, serverDCAddr, clientCCAddr, clientDCAddr, serverBwp, clientBwp)
	} else {

		singleRun(CCConn, DCConn, serverBwp, clientBwp,
			func() {
				Check(fmt.Errorf("Error, could not receive a server response, MaxTries attempted without success."))
			},
			func() {
				Check(fmt.Errorf("Error, could not fetch server results, MaxTries attempted without success."))
			})
	}
}

func findMaxBandwidth(CCConn, DCConn snet.Conn, serverCCAddr, serverDCAddr, clientCCAddr, clientDCAddr *snet.Addr,
	serverBwp, clientBwp BwtestParameters) {
	var (
		clientOldAch, serverOldAch       int64
		clientThreshold, serverThreshold int64
		serverMax, clientMax             bool
		finished                         bool
		// run is used to hold the number of the current run.
		run uint16
	)

	// Calculate from bandwidth parameters from the user
	serverBw := (serverBwp.NumPackets * serverBwp.PacketSize * 8) / int64(serverBwp.BwtestDuration/time.Second)
	clientBw := (clientBwp.NumPackets * clientBwp.PacketSize * 8) / int64(clientBwp.BwtestDuration/time.Second)

	for !finished {
		run++
		fmt.Println(strings.Repeat("#", 50))
		fmt.Println("Run: ", run)

		DCConn = resetConn(DCConn, clientDCAddr, serverDCAddr)

		// calculate the new number of packets to send based on
		serverBwp.NumPackets = (serverBw * int64(serverBwp.BwtestDuration/time.Second)) / (serverBwp.PacketSize * 8)
		clientBwp.NumPackets = (clientBw * int64(clientBwp.BwtestDuration/time.Second)) / (clientBwp.PacketSize * 8)

		fmt.Println("\nBandwidth values:")
		fmt.Printf("server->client: 	%.3f Mbps\n", float64(serverBw)/1e6)
		fmt.Printf("client->server: 	%.3f Mbps\n", float64(clientBw)/1e6)

		res, sres, failed := singleRun(CCConn, DCConn, serverBwp, clientBwp, func() {
			handleSCError(&serverMax, &clientMax, &serverBw, &clientBw,
				&serverOldAch, &clientOldAch, &serverThreshold, &clientThreshold)
		}, func() {
			handleCSError(&clientMax, &clientBw, &clientOldAch, &clientThreshold)
		})

		if failed {
			//resetCCConn()
			CCConn = resetConn(CCConn, clientCCAddr, serverCCAddr)
		} else {
			ach := 8 * serverBwp.PacketSize * res.CorrectlyReceived / int64(serverBwp.BwtestDuration/time.Second)
			handleBandwidth(&serverMax, &serverBw, &serverOldAch, &serverThreshold, ach, "server -> client")

			ach = 8 * clientBwp.PacketSize * sres.CorrectlyReceived / int64(clientBwp.BwtestDuration/time.Second)
			handleBandwidth(&clientMax, &clientBw, &clientOldAch, &clientThreshold, ach, "client -> server")
		}

		// Check if we found the maximum bandwidth for the client and the server
		finished = clientMax && serverMax
		time.Sleep(time.Second)
	}

	fmt.Println("Max server -> client available bandwidth: ", float64(serverBw)/1e6, " Mbps")
	fmt.Println("Max client -> server available bandwidth: ", float64(clientBw)/1e6, " Mbps")
	os.Exit(0)
}

// handleSCError is used in findMaxBandwidth to handle the server -> client error in a single run.
// It decreases both the server and client bandwidth, since the test failed without testing the
// client to server bandwidth.Then checks if one of them reached the minimum bandwidth.
func handleSCError(serverMax, clientMax *bool, serverBw, clientBw, serverOldAch, clientOldAch,
	serverThreshold, clientThreshold *int64) {
	fmt.Println("[Error] Server -> Client test failed: could not receive a server response," +
		" MaxTries attempted without success.")

	// if we reached the minimum bandwidth then stop the test because definitely something is wrong.
	if *serverBw == MinBandwidth || *clientBw == MinBandwidth {
		Check(fmt.Errorf("reached minimum bandwidth (5Kbps) and no response received " +
			"from the server"))
	}

	handleBandwidth(serverMax, serverBw, serverOldAch, serverThreshold, 0, "server -> client")
	handleBandwidth(clientMax, clientBw, clientOldAch, clientThreshold, 0, "client -> server")
}

// handleCSError is also used in findMaxBandwidth to handle single run error.
// Only modifies the client's bandwidth, since this mean the server to client test succeeded.
func handleCSError(clientMax *bool, clientBw, clientOldAch, clientThreshold *int64) {
	fmt.Println("[Error] Client -> Server test failed: could not fetch server results, " +
		"MaxTries attempted without success.")
	if *clientBw == MinBandwidth {
		Check(fmt.Errorf("reached minimum bandwidth (5Kbps) and no results received " +
			"from the server"))
	}
	// Don't change the server's bandwidth since its test succeeded
	handleBandwidth(clientMax, clientBw, clientOldAch, clientThreshold, 0, "client -> server")

}

// handleBandwidth increases or decreases the bandwidth to try next based on the
// achieved bandwidth (ach) and the previously achieved bandwidth (oldBw).
// We do not use loss since the link might be lossy, then the loss would not be a good metric.
// "name" is just the name of the bandwidth to reduce to print out to the user.
func handleBandwidth(isMax *bool, currentBw, oldAch, threshold *int64, ach int64, name string) {
	if *isMax {
		return
	}
	if *oldAch < ach {
		fmt.Printf("Increasing %s bandwidth...\n", name)
		*currentBw = increaseBandwidth(*currentBw, *threshold)
	} else {
		fmt.Printf("Decreasing %s bandwidth...\n", name)
		*currentBw, *threshold, *isMax = decreaseBandwidth(*currentBw, *threshold, ach, *oldAch)
	}
	*oldAch = ach
}

// increaseBandwidth returns a new bandwidth based on threshold and bandwidth values parameters. When the bandwidth is
// lower than threshold, it will be increased by 100% (doubled), otherwise, if the bandwidth is higher than the
// than the threshold then it will only be increased by 10%.
func increaseBandwidth(currentBandwidth, threshold int64) int64 {
	var newBandwidth int64

	if currentBandwidth >= threshold && threshold != 0 {
		newBandwidth = int64(float64(currentBandwidth) * 1.1)
	} else {
		newBandwidth = currentBandwidth * 2
		// Check if threshold is set and that bandwidth does not exceed it if that is the case
		// the bandwidth should be set to the threshold
		if newBandwidth > threshold && threshold != 0 {
			newBandwidth = threshold
		}
	}

	return newBandwidth
}

// decreaseBandwidth returns a new decreased bandwidth and a threshold based on the passed threshold and bandwidth
// parameters, and returns true if the returned bandwidth is the maximum achievable bandwidth.
func decreaseBandwidth(currentBandwidth, threshold, achievedBandwidth, oldAchieved int64) (newBandwidth,
	newThreshold int64, isMaxBandwidth bool) {

	// Choose the larger value between them so we don't do unnecessary slow start since we know both bandwidths are
	// achievable on that link.
	if achievedBandwidth < oldAchieved {
		newBandwidth = oldAchieved
	} else {
		// Both achieved bandwidth and oldBw are not set which means an error occurred when using those values
		if achievedBandwidth == 0 {
			newBandwidth = currentBandwidth / 2
		} else {
			newBandwidth = achievedBandwidth
		}
	}
	// Check if the bandwidth did not change for some error then reduce it by half of the current bandwidth
	if newBandwidth == currentBandwidth {
		newBandwidth = currentBandwidth / 2
	}
	// if the threshold is not set then set the threshold and bandwidth
	if currentBandwidth <= threshold || threshold == 0 {
		newThreshold = newBandwidth
	} else if currentBandwidth > threshold {
		// threshold is set and we had to decrease the Bandwidth which means we hit the max bandwidth
		isMaxBandwidth = true
	}

	// Check if we are lower than the lowest bandwidth possible if so that is the maximum achievable bandwidth on
	// that link
	if newBandwidth < MinBandwidth {
		newThreshold = MinBandwidth
		newBandwidth = MinBandwidth
		isMaxBandwidth = true
	}

	return
}

// initConns sets up the paths to the server, initializes the Control Channel
// connection, sets up the Data connection addresses, starts the Data Channel
// connection, then it updates packet size.
func initConns(serverCCAddr, serverDCAddr, clientCCAddr, clientDCAddr *snet.Addr, pathAlgo string, interactive bool) (CCConn,
	DCConn snet.Conn, err error) {
	var pathEntry *sciond.PathReplyEntry
	if !serverCCAddr.IA.Equal(clientCCAddr.IA) {
		if interactive {
			pathEntry = scionutil.ChoosePathInteractive(clientCCAddr, serverCCAddr)
		} else {
			var metric int
			if pathAlgo == "mtu" {
				metric = scionutil.MTU
			} else if pathAlgo == "shortest" {
				metric = scionutil.Shortest
			}
			pathEntry = scionutil.ChoosePathByMetric(metric, clientCCAddr, serverCCAddr)
		}
		if pathEntry == nil {
			LogFatal("No paths available to remote destination")
		}
		serverCCAddr.Path = spath.New(pathEntry.Path.FwdPath)
		_ = serverCCAddr.Path.InitOffsets()
		serverCCAddr.NextHop, _ = pathEntry.HostInfo.Overlay()
	} else {
		_ = scionutil.InitSCION(clientCCAddr)
	}

	// Control channel connection
	CCConn, err = snet.DialSCION(overlayType, clientCCAddr, serverCCAddr)
	if err != nil {
		return
	}
	// get the port used by clientCC after it bound to the dispatcher (because it might be 0)
	clientPort := uint((CCConn.LocalAddr()).(*snet.Addr).Host.L4.Port())
	serverPort := serverCCAddr.Host.L4.Port()

	//Address of client data channel (DC)
	*clientDCAddr = snet.Addr{IA: clientCCAddr.IA, Host: &addr.AppAddr{
		L3: clientCCAddr.Host.L3, L4: addr.NewL4UDPInfo(uint16(clientPort) + 1)}}
	// Address of server data channel (DC)
	*serverDCAddr = snet.Addr{IA: serverCCAddr.IA, Host: &addr.AppAddr{
		L3: serverCCAddr.Host.L3, L4: addr.NewL4UDPInfo(serverPort + 1)}}

	// Set path on data connection
	if !serverDCAddr.IA.Equal(clientDCAddr.IA) {
		serverDCAddr.Path = spath.New(pathEntry.Path.FwdPath)
		_ = serverDCAddr.Path.InitOffsets()
		serverDCAddr.NextHop, _ = pathEntry.HostInfo.Overlay()
		fmt.Printf("Client DC \tNext Hop %v\tServer Host %v\n",
			serverDCAddr.NextHop, serverDCAddr.Host)
	}

	//Data channel connection
	DCConn, err = snet.DialSCION(overlayType, clientDCAddr, serverDCAddr)
	if err != nil {
		return
	}
	// update default packet size to max MTU on the selected path
	if pathEntry != nil {
		InferedPktSize = int64(pathEntry.Path.Mtu)
	} else {
		// use default packet size when within same AS and pathEntry is not set
		InferedPktSize = DefaultPktSize
	}

	return
}

func resetConn(conn snet.Conn, localAddress, remoteAddress *snet.Addr) snet.Conn {
	var err error
	_ = conn.Close()

	// give it time to close the connection before trying to open it again
	time.Sleep(time.Millisecond * 100)

	conn, err = snet.DialSCION(overlayType, localAddress, remoteAddress)
	if err != nil {
		LogFatal("Resetting connection", "err", err)
	}
	return conn
}

// startTest runs the server to client test.
// It should be called right after setting up the flags to start the test.
// Returns the bandwidth test results and a boolean to indicate a
//// failure in the measurements (when max tries is reached).
func startTest(CCConn, DCConn snet.Conn, serverBwp, clientBwp BwtestParameters,
	receiveDone chan struct{}) (*BwtestResult, bool) {
	var tzero time.Time
	var err error

	t := time.Now()
	expFinishTimeSend := t.Add(serverBwp.BwtestDuration + MaxRTT + GracePeriodSend)
	expFinishTimeReceive := t.Add(clientBwp.BwtestDuration + MaxRTT + StragglerWaitPeriod)
	res := BwtestResult{NumPacketsReceived: -1,
		CorrectlyReceived:  -1,
		IPAvar:             -1,
		IPAmin:             -1,
		IPAavg:             -1,
		IPAmax:             -1,
		PrgKey:             clientBwp.PrgKey,
		ExpectedFinishTime: expFinishTimeReceive,
	}
	var resLock sync.Mutex
	if expFinishTimeReceive.Before(expFinishTimeSend) {
		// The receiver will close the DC connection, so it will wait long enough until the
		// sender is also done
		res.ExpectedFinishTime = expFinishTimeSend
	}

	go HandleDCConnReceive(&serverBwp, DCConn, &res, &resLock, receiveDone)

	pktbuf := make([]byte, 2000)
	pktbuf[0] = 'N' // Request for new bwtest
	n := EncodeBwtestParameters(&clientBwp, pktbuf[1:])
	l := n + 1
	n = EncodeBwtestParameters(&serverBwp, pktbuf[l:])
	l = l + n

	var numtries int64 = 0
	for numtries < MaxTries {
		if _, err = CCConn.Write(pktbuf[:l]); err != nil {
			LogFatal("[startTest] Writing to CCConn", "err", err)
		}

		if err = CCConn.SetReadDeadline(time.Now().Add(MaxRTT)); err != nil {
			LogFatal("[startTest] Setting deadline for CCConn", "err", err)
		}

		n, err = CCConn.Read(pktbuf)
		if err != nil {
			// A timeout likely happened, see if we should adjust the expected finishing time
			expFinishTimeReceive = time.Now().Add(clientBwp.BwtestDuration + MaxRTT + StragglerWaitPeriod)
			resLock.Lock()
			if res.ExpectedFinishTime.Before(expFinishTimeReceive) {
				res.ExpectedFinishTime = expFinishTimeReceive
			}
			resLock.Unlock()

			numtries++
			continue
		}
		// Remove read deadline
		if err = CCConn.SetReadDeadline(tzero); err != nil {
			LogFatal("[startTest] removing deadline for CCConn", "err", err)
		}

		if n != 2 || pktbuf[0] != 'N' {
			fmt.Println("Incorrect server response, trying again")
			time.Sleep(Timeout)
			numtries++
			continue
		}
		if pktbuf[1] != 0 {
			// The server asks us to wait for some amount of time
			time.Sleep(time.Second * time.Duration(int(pktbuf[1])))
			// Don't increase numtries in this case
			continue
		}
		// Everything was successful, exit the loop
		break
	}

	return &res, numtries == MaxTries
}

// fetchResults gets the results from the server for the client to server test.
// It should be invoked after calling startTest and HandleDCConnSend
// (See normalRun for an example).
// Returns the bandwidth test results and a boolean to indicate a
// failure in the measurements (when max tries is reached).
func fetchResults(CCConn snet.Conn, clientBwp BwtestParameters) (*BwtestResult, bool) {
	var tzero time.Time
	var err error

	pktbuf := make([]byte, 2000)
	var numtries int64 = 0
	sres := &BwtestResult{NumPacketsReceived: -1,
		CorrectlyReceived: -1,
		IPAvar:            -1,
		IPAmin:            -1,
		IPAavg:            -1,
		IPAmax:            -1,
		PrgKey:            clientBwp.PrgKey,
	}
	var n int
	for numtries < MaxTries {
		pktbuf[0] = 'R'
		copy(pktbuf[1:], clientBwp.PrgKey)
		_, err = CCConn.Write(pktbuf[:1+len(clientBwp.PrgKey)])
		Check(err)

		err = CCConn.SetReadDeadline(time.Now().Add(MaxRTT))
		Check(err)
		n, err = CCConn.Read(pktbuf)
		if err != nil {
			numtries++
			continue
		}
		// Remove read deadline
		err = CCConn.SetReadDeadline(tzero)
		Check(err)

		if n < 2 || pktbuf[0] != 'R' {
			numtries++
			continue
		}
		if pktbuf[1] != byte(0) {
			// Error case
			if pktbuf[1] == byte(127) {
				// print the results before exiting
				fmt.Println("[Error] Results could not be found or PRG key was incorrect, aborting " +
					"current test")
				// set numtries to the max so the current test does not continue and attempted bandwidth
				// is reduced
				numtries = MaxTries
				break
			}
			// pktbuf[1] contains number of seconds to wait for results
			fmt.Println("We need to sleep for", pktbuf[1], "seconds before we can get the results")
			time.Sleep(time.Duration(pktbuf[1]) * time.Second)
			// We don't increment numtries as this was not a lost packet or other communication error
			continue
		}

		var n1 int
		sres, n1, err = DecodeBwtestResult(pktbuf[2:])
		if err != nil {
			fmt.Println("Decoding error, try again")
			numtries++
			continue
		}
		if n1+2 < n {
			fmt.Println("Insufficient number of bytes received, try again")
			time.Sleep(Timeout)
			numtries++
			continue
		}
		if !bytes.Equal(clientBwp.PrgKey, sres.PrgKey) {
			fmt.Println("PRG Key returned from server incorrect, this should never happen")
			numtries++
			continue
		}

		break
	}

	return sres, numtries == MaxTries
}

// printResults prints the attempted and achieved bandwidth, loss, and the
// interarrival time of the specified results res and bandwidth test parameters bwp.
func printResults(res *BwtestResult, bwp BwtestParameters) {
	att := 8 * bwp.PacketSize * bwp.NumPackets / int64(bwp.BwtestDuration/time.Second)
	ach := 8 * bwp.PacketSize * res.CorrectlyReceived / int64(bwp.BwtestDuration/time.Second)
	fmt.Printf("Attempted bandwidth: %d bps / %.3f Mbps\n", att, float64(att)/1e6)
	fmt.Printf("Achieved bandwidth: %d bps / %.3f Mbps\n", ach, float64(ach)/1e6)
	loss := float32(bwp.NumPackets-res.CorrectlyReceived) * 100 / float32(bwp.NumPackets)
	fmt.Println("Loss rate:", loss, "%")
	variance := res.IPAvar
	average := res.IPAavg
	fmt.Printf("Interarrival time variance: %dms, average interarrival time: %dms\n",
		variance/1e6, average/1e6)
	fmt.Printf("Interarrival time min: %dms, interarrival time max: %dms\n",
		res.IPAmin/1e6, res.IPAmax/1e6)
}

// singleRun runs a single bandwidth test based in the clientBwp and serverBwp.
// The test parameters should be set before using this function.
func singleRun(CCConn, DCConn snet.Conn, serverBwp, clientBwp BwtestParameters, scError,
	csError func()) (res, sres *BwtestResult, failed bool) {
	receiveDone := make(chan struct{})
	res, failed = startTest(CCConn, DCConn, serverBwp, clientBwp, receiveDone)
	if failed {
		scError()
		return
	}

	go HandleDCConnSend(&clientBwp, DCConn)

	<-receiveDone
	fmt.Println("\nS->C results")
	printResults(res, serverBwp)

	// Fetch results from server
	sres, failed = fetchResults(CCConn, clientBwp)
	if failed {
		csError()
		return
	}

	fmt.Println("\nC->S results")
	printResults(sres, clientBwp)
	return
}
