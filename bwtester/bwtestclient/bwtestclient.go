// bwtestserver application
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
	DefaultBwtestParameters               = "3,1000,30,80kbps"
	DefaultDuration                       = 3
	DefaultPktSize                        = 1000
	DefaultPktCount                       = 30
	DefaultBW                             = 3000
	WildcardChar                          = "?"
	GracePeriodSync         time.Duration = time.Millisecond * 10
)

var (
	InferedPktSize int64
	sciondAddr     *string
	sciondFromIA   *bool
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
		serverPort      uint16
		// Address of client control channel (CC)
		clientCCAddr *snet.Addr
		// Address of server control channel (CC)
		serverCCAddr *snet.Addr
		// Control channel connection
		CCConn snet.Conn

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

		err   error
		tzero time.Time // initialized to "zero" time

		receiveDone sync.Mutex // used to signal when the HandleDCConnReceive goroutine has completed
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
		serverCCAddr.Path.InitOffsets()
		serverCCAddr.NextHop, _ = pathEntry.HostInfo.Overlay()
	} else {
		scionutil.InitSCION(clientCCAddr)
	}

	CCConn, err = snet.DialSCION(overlayType, clientCCAddr, serverCCAddr)
	Check(err)

	// get the port used by clientCC after it bound to the dispatcher (because it might be 0)
	clientPort = uint((CCConn.LocalAddr()).(*snet.Addr).Host.L4.Port())
	serverPort = serverCCAddr.Host.L4.Port()

	// Address of client data channel (DC)
	clientDCAddr = &snet.Addr{IA: clientCCAddr.IA, Host: &addr.AppAddr{
		L3: clientCCAddr.Host.L3, L4: addr.NewL4UDPInfo(uint16(clientPort) + 1)}}
	// Address of server data channel (DC)
	serverDCAddr = &snet.Addr{IA: serverCCAddr.IA, Host: &addr.AppAddr{
		L3: serverCCAddr.Host.L3, L4: addr.NewL4UDPInfo(uint16(serverPort) + 1)}}

	// Set path on data connection
	if !serverDCAddr.IA.Equal(clientDCAddr.IA) {
		serverDCAddr.Path = spath.New(pathEntry.Path.FwdPath)
		serverDCAddr.Path.InitOffsets()
		serverDCAddr.NextHop, _ = pathEntry.HostInfo.Overlay()
		fmt.Printf("Client DC \tNext Hop %v\tServer Host %v\n",
			serverDCAddr.NextHop, serverDCAddr.Host)
	}

	// Data channel connection
	DCConn, err = snet.DialSCION(overlayType, clientDCAddr, serverDCAddr)
	Check(err)

	// update default packet size to max MTU on the selected path
	if pathEntry != nil {
		InferedPktSize = int64(pathEntry.Path.Mtu)
	} else {
		// use default packet size when within same AS and pathEntry is not set
		InferedPktSize = DefaultPktSize
	}
	if !flagset["cs"] && flagset["sc"] { // Only one direction set, used same for reverse
		clientBwpStr = serverBwpStr
		fmt.Println("Only sc parameter set, using same values for cs")
	}
	clientBwp = parseBwtestParameters(clientBwpStr)
	clientBwp.Port = uint16(clientPort + 1)
	if !flagset["sc"] && flagset["cs"] { // Only one direction set, used same for reverse
		serverBwpStr = clientBwpStr
		fmt.Println("Only cs parameter set, using same values for sc")
	}
	serverBwp = parseBwtestParameters(serverBwpStr)
	serverBwp.Port = uint16(serverPort + 1)
	fmt.Println("\nTest parameters:")
	fmt.Println("clientDCAddr -> serverDCAddr", clientDCAddr, "->", serverDCAddr)
	fmt.Printf("client->server: %d seconds, %d bytes, %d packets\n",
		int(clientBwp.BwtestDuration/time.Second), clientBwp.PacketSize, clientBwp.NumPackets)
	fmt.Printf("server->client: %d seconds, %d bytes, %d packets\n",
		int(serverBwp.BwtestDuration/time.Second), serverBwp.PacketSize, serverBwp.NumPackets)

	t := time.Now()
	expFinishTimeSend := t.Add(serverBwp.BwtestDuration + MaxRTT + GracePeriodSend)
	expFinishTimeReceive := t.Add(clientBwp.BwtestDuration + MaxRTT + StragglerWaitPeriod)
	res := BwtestResult{-1, -1, -1, -1, -1, -1, clientBwp.PrgKey, expFinishTimeReceive}
	var resLock sync.Mutex
	if expFinishTimeReceive.Before(expFinishTimeSend) {
		// The receiver will close the DC connection, so it will wait long enough until the
		// sender is also done
		res.ExpectedFinishTime = expFinishTimeSend
	}

	receiveDone.Lock()
	go HandleDCConnReceive(&serverBwp, DCConn, &res, &resLock, &receiveDone)

	pktbuf := make([]byte, 2000)
	pktbuf[0] = 'N' // Request for new bwtest
	n := EncodeBwtestParameters(&clientBwp, pktbuf[1:])
	l := n + 1
	n = EncodeBwtestParameters(&serverBwp, pktbuf[l:])
	l = l + n

	var numtries int64 = 0
	for numtries < MaxTries {
		_, err = CCConn.Write(pktbuf[:l])
		Check(err)

		err = CCConn.SetReadDeadline(time.Now().Add(MaxRTT))
		Check(err)
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
		err = CCConn.SetReadDeadline(tzero)
		Check(err)

		if n != 2 {
			fmt.Println("Incorrect server response, trying again")
			time.Sleep(Timeout)
			numtries++
			continue
		}
		if pktbuf[0] != 'N' {
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

	if numtries == MaxTries {
		Check(fmt.Errorf("Error, could not receive a server response, MaxTries attempted without success."))
	}

	go HandleDCConnSend(&clientBwp, DCConn)

	receiveDone.Lock()

	fmt.Println("\nS->C results")
	att := 8 * serverBwp.PacketSize * serverBwp.NumPackets / int64(serverBwp.BwtestDuration/time.Second)
	ach := 8 * serverBwp.PacketSize * res.CorrectlyReceived / int64(serverBwp.BwtestDuration/time.Second)
	fmt.Printf("Attempted bandwidth: %d bps / %.2f Mbps\n", att, float64(att)/1000000)
	fmt.Printf("Achieved bandwidth: %d bps / %.2f Mbps\n", ach, float64(ach)/1000000)
	fmt.Println("Loss rate:", (serverBwp.NumPackets-res.CorrectlyReceived)*100/serverBwp.NumPackets, "%")
	variance := res.IPAvar
	average := res.IPAavg
	fmt.Printf("Interarrival time variance: %dms, average interarrival time: %dms\n",
		variance/1e6, average/1e6)
	fmt.Printf("Interarrival time min: %dms, interarrival time max: %dms\n",
		res.IPAmin/1e6, res.IPAmax/1e6)

	// Fetch results from server
	numtries = 0
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

		if n < 2 {
			numtries++
			continue
		}
		if pktbuf[0] != 'R' {
			numtries++
			continue
		}
		if pktbuf[1] != byte(0) {
			// Error case
			if pktbuf[1] == byte(127) {
				Check(fmt.Errorf("Results could not be found or PRG key was incorrect, abort"))
			}
			// pktbuf[1] contains number of seconds to wait for results
			fmt.Println("We need to sleep for", pktbuf[1], "seconds before we can get the results")
			time.Sleep(time.Duration(pktbuf[1]) * time.Second)
			// We don't increment numtries as this was not a lost packet or other communication error
			continue
		}

		sres, n1, err := DecodeBwtestResult(pktbuf[2:])
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
		fmt.Println("\nC->S results")
		att = 8 * clientBwp.PacketSize * clientBwp.NumPackets / int64(clientBwp.BwtestDuration/time.Second)
		ach = 8 * clientBwp.PacketSize * sres.CorrectlyReceived / int64(clientBwp.BwtestDuration/time.Second)
		fmt.Printf("Attempted bandwidth: %d bps / %.2f Mbps\n", att, float64(att)/1000000)
		fmt.Printf("Achieved bandwidth: %d bps / %.2f Mbps\n", ach, float64(ach)/1000000)
		fmt.Println("Loss rate:", (clientBwp.NumPackets-sres.CorrectlyReceived)*100/clientBwp.NumPackets, "%")
		variance := sres.IPAvar
		average := sres.IPAavg
		fmt.Printf("Interarrival time variance: %dms, average interarrival time: %dms\n",
			variance/1e6, average/1e6)
		fmt.Printf("Interarrival time min: %dms, interarrival time max: %dms\n",
			sres.IPAmin/1e6, sres.IPAmax/1e6)
		return
	}

	fmt.Println("Error, could not fetch server results, MaxTries attempted without success.")
}
