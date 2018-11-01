package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chaehni/scion-http/http"
	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

func main() {

	var remote = flag.String("remote", "", "The address on which the server will be listening")
	var local = flag.String("local", "", "The address on which the server will be listening")
	var interactive = flag.Bool("i", false, "Wether to use interactive mode for path selection")

	flag.Parse()

	rAddr, _ := snet.AddrFromString(*remote)
	lAddr, _ := snet.AddrFromString(*local)
	sciondPath := utils.GetSCIOND()
	dispatcherPath := utils.GetDispatcher()

	if *interactive {
		err := snet.Init(lAddr.IA, sciondPath, dispatcherPath)
		if err != nil {
			log.Fatal(err)
		}
		pathEntry := choosePath(lAddr, rAddr)
		if pathEntry == nil {
			log.Fatal("No paths available to remote destination")
		}
		rAddr.Path = spath.New(pathEntry.Path.FwdPath)
		rAddr.Path.InitOffsets()
		rAddr.NextHopHost = pathEntry.HostInfo.Host()
		rAddr.NextHopPort = pathEntry.HostInfo.Port
	}

	// Make a map from URL to *snet.Addr
	dns := make(map[string]*snet.Addr)
	dns["testserver.com"] = rAddr

	// Create a standard server with our custom RoundTripper
	c := &http.Client{
		Transport: &shttp.Transport{
			DNS:   dns,
			LAddr: lAddr,
		},
	}

	// Make a get request
	start := time.Now()
	resp, err := c.Get("http://testserver.com/download")
	if err != nil {
		log.Fatal("GET request failed: ", err)
	}
	defer resp.Body.Close()
	end := time.Now()

	log.Printf("GET request succeeded in %v seconds", end.Sub(start).Seconds())
	printResponse(resp, true)

	// Make another request with a new client
	c = &http.Client{
		Transport: &shttp.Transport{
			DNS:   dns,
			LAddr: lAddr,
		},
	}

	start = time.Now()
	resp, err = c.Post("http://testserver.com/upload", "text/html", strings.NewReader("Sample payload for POST request"))
	if err != nil {
		log.Fatal("POST request failed: ", err)
	}
	defer resp.Body.Close()
	end = time.Now()

	log.Printf("2st POST request succeeded in %v seconds", end.Sub(start).Seconds())
	printResponse(resp, false)
}

func printResponse(resp *http.Response, hasBody bool) {
	log.Println("Status: ", resp.Status)
	log.Println("Content-Length: ", resp.ContentLength)
	log.Println("Content-Type: ", resp.Header.Get("Content-Type"))
	if !hasBody {
		fmt.Println("\n\n")
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}
	log.Println("Body: ", string(body))
	fmt.Println("\n\n")
}

func choosePath(local *snet.Addr, remote *snet.Addr) *sciond.PathReplyEntry {

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
	return entry
}
