package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/elwin/transmit2/scion"

	"github.com/elwin/transmit2/mode"

	ftp "github.com/elwin/transmit2/client"
)

var (
	client = flag.String("local", "", "Local host (including port)")
	remote = flag.String("remote", "", "Remote host (including port)")
)

const (
	sizeUnit = 1024 * 1024 // MB
)

func main() {

	flag.Parse()
	if *remote == "" {
		log.Fatal("Please provide a remote address with -remote")
	}

	if err := run(); err != nil {
		fmt.Println(err)
	}

}

type test struct {
	mode        byte
	parallelism int
	payload     int // in MB
	blockSize   int
	duration    time.Duration
	selector    scion.PathSelector
	bandwidth   float64
}

func (test *test) String() string {
	if test.mode == mode.Stream {
		return fmt.Sprintf("Stream with %d KB: %s", test.payload, test.duration)
	} else {
		return fmt.Sprintf("Extended (streams: %d, bs: %d) with %d MB: %s", test.parallelism, test.blockSize, test.payload, test.duration)
	}
}

func writeToCsv(results []*test) {
	w := csv.NewWriter(os.Stderr)
	header := []string{"mode", "parallelism", "payload (MB)", "block_size", "duration", "bandwidth"}
	if err := w.Write(header); err != nil {
		log.Fatal(err)
	}
	for _, result := range results {
		record := []string{
			string(result.mode),
			strconv.Itoa(result.parallelism),
			strconv.Itoa(result.payload),
			strconv.Itoa(result.blockSize),
			strconv.FormatFloat(result.duration.Seconds(), 'f', -1, 64),
			strconv.FormatFloat(float64(result.payload)/result.duration.Seconds(), 'f', -1, 64),
		}
		if err := w.Write(record); err != nil {
			log.Fatal(err)
		}
	}

	w.Flush()
}

func run() error {

	extended := []rune{mode.ExtendedBlockMode, mode.Stream}
	parallelisms := []int{1, 2, 4, 8, 16, 32}
	payloads := []int{8}
	blocksizes := []int{4096}
	selection := []scion.PathSelector{scion.InteractivePathSelector}

	var tests []*test
	for _, m := range extended {
		for _, payload := range payloads {
			for _, selector := range selection {
				if m == mode.Stream {
					test := &test{
						mode:     mode.Stream,
						payload:  payload,
						selector: scion.DefaultPathSelector,
					}
					tests = append(tests, test)
				} else {
					for _, blocksize := range blocksizes {
						for _, parallelism := range parallelisms {
							test := &test{
								mode:        mode.ExtendedBlockMode,
								parallelism: parallelism,
								payload:     payload,
								blockSize:   blocksize,
								selector:    selector,
							}
							tests = append(tests, test)
						}
					}
				}
			}
		}
	}

	conn, err := ftp.Dial(*client, *remote)
	defer conn.Quit()
	if err = conn.Login("admin", "123456"); err != nil {
		return err
	}

	// Warm-up
	response, err := conn.Retr(strconv.Itoa(tests[0].payload * sizeUnit))
	if err != nil {
		log.Fatal("failed to retrieve file", err)
	} else {
		_, err = io.Copy(ioutil.Discard, response)
		if err != nil {
			log.Fatal("failed to copy data", err)
		}
		response.Close()
	}

	for _, test := range tests {
		if err != nil {
			return err
		}

		err = conn.Mode(test.mode)
		if err != nil {
			return err
		}

		if test.mode == mode.ExtendedBlockMode {
			err = conn.SetRetrOpts(test.parallelism, test.blockSize)
			if err != nil {
				return err
			}
		}

		start := time.Now()
		response, err := conn.Retr(strconv.Itoa(test.payload * sizeUnit))
		if err != nil {
			return err
		}

		n, err := io.Copy(ioutil.Discard, response)
		if err != nil {
			return err
		}
		if int(n) != test.payload*sizeUnit {
			return fmt.Errorf("failed to read correct number of bytes, expected %d but got %d", test.payload*sizeUnit, n)
		}
		response.Close()

		test.duration += time.Since(start)

		fmt.Print(".")
	}
	fmt.Println()

	for _, test := range tests {
		fmt.Println(test)
	}

	fmt.Println("--------------")

	writeToCsv(tests)

	return nil
}
