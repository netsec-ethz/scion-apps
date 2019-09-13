// Copyright 2018 ETH Zurich
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
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
)

// Check just ensures the error is nil, or complains and quits
func Check(e error) {
	if e != nil {
		fmt.Println("Fatal error. Exiting.", "err", e)
		os.Exit(1)
	}
}

func main() {
	var err error
	var clientCCAddrStr string
	var serverCCAddrStr string
	dispatcherPath := scionutil.GetDefaultDispatcher()
	// get local and remote addresses from program arguments:
	flag.StringVar(&clientCCAddrStr, "local", "", "Local SCION Address (e.g. 17-ffaa:1:1,[127.0.0.1]:0)")
	flag.StringVar(&serverCCAddrStr, "remote", "", "Remote SCION Address (e.g. 17-ffaa:1:1,[127.0.0.1]:12345)")
	flag.Parse()
	if len(clientCCAddrStr) == 0 {
		clientCCAddrStr, err = scionutil.GetLocalhostString()
		Check(err)
	}
	if len(serverCCAddrStr) == 0 {
		Check(fmt.Errorf("Error, remote address needs to be specified with -remote"))
	}
	clientCCAddr, err := snet.AddrFromString(clientCCAddrStr)
	Check(err)
	serverCCAddr, err := snet.AddrFromString(serverCCAddrStr)
	Check(err)
	// get the daemon socket file path:
	sciondPath := sciond.GetDefaultSCIONDPath(nil)
	// initialize SCION
	err = snet.Init(clientCCAddr.IA, sciondPath, dispatcherPath)
	Check(err)

	if !serverCCAddr.IA.Equal(clientCCAddr.IA) {
		// query paths from here to there:
		pathMgr := snet.DefNetwork.PathResolver()
		pathSet := pathMgr.Query(context.Background(), clientCCAddr.IA, serverCCAddr.IA, sciond.PathReqFlags{})
		if len(pathSet) == 0 {
			Check(fmt.Errorf("No paths"))
		}
		// print all paths. Also pick one path. Here we chose the path with least hops:
		i := 0
		minLength, argMinPath := 999, (*sciond.PathReplyEntry)(nil)
		fmt.Println("Available paths:")
		for _, path := range pathSet {
			fmt.Printf("[%2d] %d %s\n", i, len(path.Entry.Path.Interfaces)/2, path.Entry.Path.String())
			if len(path.Entry.Path.Interfaces) < minLength {
				minLength = len(path.Entry.Path.Interfaces)
				argMinPath = path.Entry
			}
			i++
		}
		fmt.Println("Chosen path:", argMinPath.Path.String())
		// we need to copy the path to the destination (destination is the whole selected path)
		serverCCAddr.Path = spath.New(argMinPath.Path.FwdPath)
		serverCCAddr.Path.InitOffsets()
		serverCCAddr.NextHop, _ = argMinPath.HostInfo.Overlay()
		// get a connection object using that path:
	}
	conn, err := snet.DialSCION("udp4", clientCCAddr, serverCCAddr)
	Check(err)
	defer conn.Close()
	// when we have set our connection up, we just use it. Write some content:
	// you could visualize the packet(s) with e.g. sudo tcpdump -i any -n -A -w - |grep -a 'hello world'
	nBytes, err := conn.Write([]byte("hello world"))
	Check(err)
	fmt.Printf("Done. Wrote %d bytes.\n", nBytes)
}
