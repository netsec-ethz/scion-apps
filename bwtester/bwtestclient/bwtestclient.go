// Copyright 2020 ETH Zurich
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
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"math"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/netsec-ethz/scion-apps/bwtester/bwtest"
	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

const (
	DefaultBwtestParameters = "3,1000,30,80kbps"
	DefaultDuration         = 3
	DefaultPktSize          = 1000
	DefaultPktCount         = 30
	DefaultBW               = 3000
	WildcardChar            = "?"

	MaxTries               = 5 // Number of times to try to reach server
	Timeout  time.Duration = time.Millisecond * 500
	MaxRTT   time.Duration = time.Millisecond * 1000
)

func prepareAESKey() []byte {
	key := make([]byte, 16)
	_, err := rand.Read(key) // guaranteed full read iff err != nil
	bwtest.Check(err)
	return key
}

func printUsage() {
	fmt.Println("Usage of bwtestclient:")
	flag.PrintDefaults()

	fmt.Println("")
	fmt.Println("Test parameters:")
	fmt.Println("\t-cs and -sc specify time duration (seconds), packet size (bytes), number of packets, and target bandwidth.")
	fmt.Println("\tThe question mark character ? can be used as wildcard when setting the test parameters " +
		"and its value is computed according to the other parameters. When more than one wilcard is used, " +
		"all but the last one are set to the default values, e.g. ?,1000,?,5Mbps will run the test for the " +
		"default duration and send as many packets as required to reach a bandwidth of 5 Mbps with the given " +
		"packet size.")
	fmt.Println("\tSupported bandwidth unit prefixes are: none (e.g. 1500bps for 1.5kbps), k, M, G, T.")
	fmt.Println("\tYou can also only set the target bandwidth, e.g. -cs 1Mbps")
	fmt.Println("\tWhen only the cs or sc flag is set, the other flag is set to the same value.")
}

// Input format (time duration,packet size,number of packets,target bandwidth), no spaces, question mark ? is wildcard
// The value of the wildcard is computed from the other values, if more than one wildcard is used,
// all but the last one are set to the defaults values
func parseBwtestParameters(s string, defaultPktSize int64) (bwtest.Parameters, error) {
	if !strings.Contains(s, ",") {
		// Using simple bandwidth setting with all defaults except bandwidth
		s = "?,?,?," + s
	}
	a := strings.Split(s, ",")
	if len(a) != 4 {
		usageErr("incorrect number of arguments, need 4 values for bwtestparameters. " +
			"You can use ? as wildcard, e.g. " + DefaultBwtestParameters)
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
			a2 = getPacketSize(a[1], defaultPktSize)
			a3 = getPacketCount(a[2])
			a4 = parseBandwidth(a[3])
			a1 = (a2 * 8 * a3) / a4
			if time.Second*time.Duration(a1) > bwtest.MaxDuration {
				fmt.Printf("Duration exceeds max: %v > %v, using default value %d\n",
					a1, bwtest.MaxDuration/time.Second, DefaultDuration)
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
			a2 = defaultPktSize
		}
	} else {
		a2 = getPacketSize(a[1], defaultPktSize)
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
		expected := a2 * a3 * 8 / a1
		leeway := 8 * a2
		lo := expected - leeway
		hi := expected + leeway
		if a4 < lo || a4 > hi {
			return bwtest.Parameters{},
				fmt.Errorf("computed target bandwidth does not match parameters, "+
					"use wildcard or specify correct bandwidth, expected %d-%d, provided %d",
					lo, hi, a4)
		}
	}
	key := prepareAESKey()
	return bwtest.Parameters{
		BwtestDuration: time.Second * time.Duration(a1),
		PacketSize:     a2,
		NumPackets:     a3,
		PrgKey:         key,
		Port:           0,
	}, nil
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
	case "M":
		m = 1e6
	case "G":
		m = 1e9
	case "T":
		m = 1e12
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
	if time.Second*time.Duration(a1) > bwtest.MaxDuration {
		usageErr(fmt.Sprintf("duration exceeds max: %d > %d", a1, bwtest.MaxDuration/time.Second))
	}
	return a1
}

func getPacketSize(size string, defaultPktSize int64) int64 {
	a2, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		fmt.Printf("Invalid packet size %v provided, using default value %d\n", a2, defaultPktSize)
		a2 = defaultPktSize
	}

	if a2 < bwtest.MinPacketSize {
		a2 = bwtest.MinPacketSize
	}
	if a2 > bwtest.MaxPacketSize {
		a2 = bwtest.MaxPacketSize
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

func usageErr(msg string) {
	printUsage()
	if msg != "" {
		fmt.Println("\nError:", msg)
	}
	os.Exit(2)
}

func checkUsageErr(err error) {
	if err != nil {
		usageErr(err.Error())
	}
}

func main() {
	var (
		local        pan.IPPortValue
		serverCCAddr pan.UDPAddr
		clientBwpStr string
		serverBwpStr string
		interactive  bool
		sequence     string
		preference   string
	)

	flag.Usage = printUsage
	flag.Var(&local, "local", "Local address")
	flag.Var(&serverCCAddr, "s", "Server SCION Address")
	flag.StringVar(&serverBwpStr, "sc", DefaultBwtestParameters, "Server->Client test parameter")
	flag.StringVar(&clientBwpStr, "cs", DefaultBwtestParameters, "Client->Server test parameter")
	flag.BoolVar(&interactive, "i", false, "Interactive path selection, prompt to choose path")
	flag.StringVar(&sequence, "sequence", "", "Sequence of space separated hop predicates to specify path")
	flag.StringVar(&preference, "preference", "", "Preference sorting order for paths. "+
		"Comma-separated list of available sorting options: "+
		strings.Join(pan.AvailablePreferencePolicies, "|"))

	flag.Parse()
	flagset := make(map[string]bool)
	// record if flags were set or if default value was used
	flag.Visit(func(f *flag.Flag) { flagset[f.Name] = true })

	if flag.NFlag() == 0 {
		usageErr("")
	}
	if !serverCCAddr.IsValid() {
		usageErr("server address needs to be specified with -s")
	}
	policy, err := pan.PolicyFromCommandline(sequence, preference, interactive)
	checkUsageErr(err)

	// use default packet size when within same AS
	inferedPktSize := int64(DefaultPktSize)
	// update default packet size to max MTU on the selected path
	// TODO(matzf): evaluate policy, set pkt size to MTU of most preferred path,
	//              append filter to policy to allow only paths with MTU >= pkt size.
	/*if path != nil {
		inferedPktSize = int64(path.Metadata().MTU)
	}*/
	if !flagset["cs"] && flagset["sc"] { // Only one direction set, used same for reverse
		clientBwpStr = serverBwpStr
		fmt.Println("Only sc parameter set, using same values for cs")
	}
	clientBwp, err := parseBwtestParameters(clientBwpStr, inferedPktSize)
	checkUsageErr(err)
	if !flagset["sc"] && flagset["cs"] { // Only one direction set, used same for reverse
		serverBwpStr = clientBwpStr
		fmt.Println("Only cs parameter set, using same values for sc")
	}
	serverBwp, err := parseBwtestParameters(serverBwpStr, inferedPktSize)
	checkUsageErr(err)
	fmt.Println("\nTest parameters:")
	fmt.Printf("client->server: %d seconds, %d bytes, %d packets\n",
		int(clientBwp.BwtestDuration/time.Second), clientBwp.PacketSize, clientBwp.NumPackets)
	fmt.Printf("server->client: %d seconds, %d bytes, %d packets\n",
		int(serverBwp.BwtestDuration/time.Second), serverBwp.PacketSize, serverBwp.NumPackets)

	clientRes, serverRes, err := runBwtest(local.Get(), serverCCAddr, policy, clientBwp, serverBwp)
	bwtest.Check(err)

	fmt.Println("\nS->C results")
	printBwtestResult(serverBwp, clientRes)
	fmt.Println("\nC->S results")
	printBwtestResult(clientBwp, serverRes)
}

// runBwtest runs the bandwidth test with the given parameters against the server at serverCCAddr.
func runBwtest(local netip.AddrPort, serverCCAddr pan.UDPAddr, policy pan.Policy,
	clientBwp, serverBwp bwtest.Parameters) (clientRes, serverRes bwtest.Result, err error) {

	// Control channel connection
	ccSelector := pan.NewDefaultSelector()
	ccConn, err := pan.DialUDP(context.Background(), local, serverCCAddr, pan.WithPolicy(policy), pan.WithSelector(ccSelector))
	if err != nil {
		return
	}

	dcLocal := netip.AddrPortFrom(local.Addr(), 0)
	// Address of server data channel (DC)
	serverDCAddr := serverCCAddr.WithPort(serverCCAddr.Port + 1)

	// Data channel connection
	dcConn, err := pan.DialUDP(context.Background(), dcLocal, serverDCAddr, pan.WithPolicy(policy))
	if err != nil {
		return
	}
	clientDCAddr := dcConn.LocalAddr().(pan.UDPAddr)
	// DC ports are passed in the request
	clientBwp.Port = clientDCAddr.Port
	serverBwp.Port = serverDCAddr.Port

	// Start receiver before even sending the request so it will be ready.
	receiveRes := make(chan bwtest.Result, 1)
	go func() {
		receiveRes <- bwtest.HandleDCConnReceive(serverBwp, dcConn)
	}()

	// Send the request; when this finishes, the server may have already started blasting.
	err = requestNewBwtest(ccConn, clientBwp, serverBwp)
	if err != nil {
		return
	}
	startTime := time.Now()
	finishTimeReceive := startTime.Add(serverBwp.BwtestDuration + bwtest.StragglerWaitPeriod)
	finishTimeSend := startTime.Add(clientBwp.BwtestDuration + bwtest.GracePeriodSend)
	if err = dcConn.SetReadDeadline(finishTimeReceive); err != nil {
		dcConn.Close()
		return
	}
	if err = dcConn.SetWriteDeadline(finishTimeSend); err != nil {
		dcConn.Close()
		return
	}

	// Pin DC to path used for request
	if serverDCAddr.IA != clientDCAddr.IA {
		dcConn.SetPolicy(pan.Pinned{ccSelector.Path().Fingerprint})
	}

	// Start blasting client->server
	err = bwtest.HandleDCConnSend(clientBwp, dcConn)
	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		dcConn.Close()
		return
	}

	// Wait until receive is done as well
	clientRes = <-receiveRes
	dcConn.Close()

	serverRes, err = requestResults(ccConn, clientBwp.PrgKey)
	return
}

// requestNewBwtest makes a new bandwidth test request at the server.
// Returns nil once the server has accepted the request and an error otherwise.
func requestNewBwtest(ccConn net.Conn, clientBwp, serverBwp bwtest.Parameters) error {
	request := make([]byte, 2000)
	request[0] = 'N' // Request for new bwtest
	nc, err := bwtest.EncodeParameters(clientBwp, request[1:])
	if err != nil {
		return fmt.Errorf("encoding client->server parameters: %w", err)
	}
	ns, err := bwtest.EncodeParameters(serverBwp, request[1+nc:])
	if err != nil {
		return fmt.Errorf("encoding server->client parameters: %w", err)
	}
	request = request[:1+nc+ns]

	response := make([]byte, 3)
	for numtries := 0; numtries < MaxTries; {
		_, err := ccConn.Write(request)
		if err != nil {
			return err
		}

		err = ccConn.SetReadDeadline(time.Now().Add(MaxRTT))
		if err != nil {
			return err
		}
		n, err := ccConn.Read(response)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			numtries++
			continue
		} else if err != nil {
			return err
		}

		if n != 2 || response[0] != 'N' {
			fmt.Println("Incorrect server response, trying again")
			time.Sleep(Timeout)
			numtries++
			continue
		}
		if response[1] != 0 {
			// The server asks us to wait for some amount of time
			time.Sleep(time.Second * time.Duration(int(response[1])))
			// Don't increase numtries in this case
			continue
		}

		// Everything was successful, exit the loop
		return nil
	}

	return fmt.Errorf("could not receive a server response, MaxTries attempted without success")
}

func requestResults(ccConn net.Conn, prgKey []byte) (bwtest.Result, error) {
	// Fetch results from server
	req := make([]byte, 1+len(prgKey))
	req[0] = 'R'
	copy(req[1:], prgKey)

	response := make([]byte, 2000)

	for numtries := 0; numtries < MaxTries; {
		_, err := ccConn.Write(req)
		if err != nil {
			return bwtest.Result{}, err
		}

		err = ccConn.SetReadDeadline(time.Now().Add(MaxRTT))
		if err != nil {
			return bwtest.Result{}, err
		}
		n, err := ccConn.Read(response)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			numtries++
			continue
		} else if err != nil {
			return bwtest.Result{}, err
		}

		if n < 2 {
			numtries++
			continue
		}
		if response[0] != 'R' {
			numtries++
			continue
		}
		if response[1] == byte(127) {
			return bwtest.Result{}, fmt.Errorf("results could not be found or PRG key was incorrect")
		} else if response[1] != byte(0) {
			// pktbuf[1] contains number of seconds to wait for results
			fmt.Println("We need to sleep for", response[1], "seconds before we can get the results")
			time.Sleep(time.Duration(response[1]) * time.Second)
			// We don't increment numtries as this was not a lost packet or other communication error
			continue
		}

		res, n1, err := bwtest.DecodeResult(response[2:])
		if err != nil {
			fmt.Println("Decoding error, try again")
			numtries++
			continue
		}
		if n1+2 < n {
			fmt.Println("Trailing bytes in response, try again")
			time.Sleep(Timeout)
			numtries++
			continue
		}
		if !bytes.Equal(prgKey, res.PrgKey) {
			fmt.Println("PRG Key returned from server incorrect, this should never happen")
			numtries++
			continue
		}
		return res, nil
	}
	return bwtest.Result{}, fmt.Errorf("could not fetch server results, MaxTries attempted without success")
}

func printBwtestResult(bwp bwtest.Parameters, res bwtest.Result) {
	att := 8 * bwp.PacketSize * bwp.NumPackets / int64(bwp.BwtestDuration/time.Second)
	ach := 8 * bwp.PacketSize * res.CorrectlyReceived / int64(bwp.BwtestDuration/time.Second)
	fmt.Printf("Attempted bandwidth: %d bps / %.2f Mbps\n", att, float64(att)/1000000)
	fmt.Printf("Achieved bandwidth: %d bps / %.2f Mbps\n", ach, float64(ach)/1000000)
	loss := 0.0
	if bwp.NumPackets > 0 {
		loss = float64(bwp.NumPackets-res.CorrectlyReceived) * 100.0 / float64(bwp.NumPackets)
	}
	fmt.Printf("Loss rate: %.1f%%\n", loss)
	fmt.Printf("Interarrival time min/avg/max/mdev = %.3f/%.3f/%.3f/%.3f ms\n",
		float64(res.IPAmin)/1e6,
		float64(res.IPAavg)/1e6,
		float64(res.IPAmax)/1e6,
		math.Sqrt(float64(res.IPAvar)/1e6),
	)
}
