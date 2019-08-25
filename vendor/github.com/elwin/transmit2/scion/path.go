package scion

import (
	"context"
	"fmt"

	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
)

func setupPath(local, remote snet.Addr) error {
	if !remote.IA.Eq(local.IA) {
		pathEntry := choosePath(local, remote)
		if pathEntry == nil {
			return fmt.Errorf("no paths available to remote destination")
		}
		remote.Path = spath.New(pathEntry.Path.FwdPath)
		remote.Path.InitOffsets()
		remote.NextHop, _ = pathEntry.HostInfo.Overlay()
	}

	return nil
}

func choosePath(local, remote snet.Addr) *sciond.PathReplyEntry {
	var paths []*sciond.PathReplyEntry
	var pathIndex uint64

	pathMgr := snet.DefNetwork.PathResolver()
	pathSet := pathMgr.Query(context.Background(), local.IA, remote.IA)

	if len(pathSet) == 0 {
		return nil
	}
	for _, p := range pathSet {
		paths = append(paths, p.Entry)
	}

	/*
		if interactive {
			fmt.Printf("Available paths to %v\n", remote.IA)
			for i := range paths {
				fmt.Printf("[%2d] %s\n", i, paths[i].Path.String())
			}
			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Printf("Choose path: ")
				pathIndexStr, _ := reader.ReadString('\n')
				var err error
				pathIndex, err = strconv.ParseUint(pathIndexStr[:len(pathIndexStr)-1], 10, 64)
				if err == nil && int(pathIndex) < len(paths) {
					break
				}
				fmt.Fprintf(os.Stderr, "ERROR: Invalid path index, valid indices range: [0, %v]\n",
					len(paths))
			}
		}
	*/

	fmt.Printf("Using path:\n  %s\n", paths[pathIndex].Path.String())
	return paths[pathIndex]
}
