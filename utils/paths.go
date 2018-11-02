package utils

import(
	"log"
	"fmt"
	"os"
	"strconv"
	"bufio"

	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

func ChoosePath(local *snet.Addr, remote *snet.Addr, sciondPath string, dispatcherPath string) {

	err := snet.Init(local.IA, sciondPath, dispatcherPath)
	if err != nil {
		log.Fatal(err)
	}

	pathMgr := snet.DefNetwork.PathResolver()
	pathSet := pathMgr.Query(local.IA, remote.IA)
	var appPaths []*spathmeta.AppPath
	var selectedPath *spathmeta.AppPath

	if len(pathSet) == 0 {
		log.Fatal("No paths available to remote destination")
	}

	fmt.Printf("Available paths to %v\n", remote.IA)
	i := 0
	for _, path := range pathSet {
		appPaths = append(appPaths, path)
		fmt.Printf("[%2d] %s\n", i, path.Entry.Path.String())
		i++
	}

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
		fmt.Printf("Error: Invalid path index %v, valid indices range: [0,  %v]\n", pathIndex, len(appPaths)-1)
	}

	entry := selectedPath.Entry
	fmt.Printf("Using path:\n %s\n", entry.Path.String())

	remote.Path = spath.New(entry.Path.FwdPath)
	remote.Path.InitOffsets()
	remote.NextHopHost = entry.HostInfo.Host() 
	remote.NextHopPort = entry.HostInfo.Port
}