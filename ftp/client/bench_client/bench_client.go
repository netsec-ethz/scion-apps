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

	"github.com/netsec-ethz/scion-apps/ftp/scion"

	"github.com/netsec-ethz/scion-apps/ftp/mode"

	ftp "github.com/netsec-ethz/scion-apps/ftp/client"
)

var (
	client = flag.String("local", "", "Local host (including port)")
	remote = flag.String("remote", "", "Remote host (including port)")
)

const (
	sizeUnit = 1000 * 1000 // MB
	sleep    = 5 * time.Second
	maxPaths = 100
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
	pathNumber  int
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
	header := []string{"mode", "parallelism", "payload (MB)", "block_size (KB)", "path_number", "duration (s)", "bandwidth (MB/s)"}
	if err := w.Write(header); err != nil {
		log.Fatal(err)
	}
	for _, result := range results {
		record := []string{
			string(result.mode),
			strconv.Itoa(result.parallelism),
			strconv.Itoa(result.payload),
			strconv.Itoa(result.blockSize),
			strconv.Itoa(result.pathNumber),
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
	payloads := []int{1}
	blocksizes := []int{4096}
	rotator := scion.NewRotator(maxPaths)
	static := scion.NewStaticSelector()
	selection := []scion.PathSelector{static.PathSelector, rotator.PathSelector}

	var tests = make([]*test, 0)
	for _, m := range extended {
		for _, payload := range payloads {
			if m == mode.Stream {
				test := &test{
					mode:     mode.Stream,
					payload:  payload,
					selector: static.PathSelector,
				}
				tests = append(tests, test)
			} else {
				for _, blocksize := range blocksizes {
					for _, parallelism := range parallelisms {
						if parallelism == 1 {
							test := &test{
								mode:        mode.ExtendedBlockMode,
								parallelism: parallelism,
								payload:     payload,
								blockSize:   blocksize,
								selector:    static.PathSelector,
							}
							tests = append(tests, test)
						} else {
							for _, selector := range selection {
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
	}

	fmt.Printf("%d Testcases\n", len(tests))

	conn, err := ftp.Dial(*client, *remote)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Quit() }()
	if err = conn.Login("admin", "123456"); err != nil {
		return err
	}

	// Warm-up
	if err = conn.Mode(mode.ExtendedBlockMode); err != nil {
		return err
	}
	if err = conn.SetRetrOpts(8, 4096); err != nil {
		return err
	}
	response, err := conn.Retr(strconv.Itoa(tests[0].payload * sizeUnit))
	if err != nil {
		log.Fatal("failed to retrieve file", err)
	} else {
		_, err = io.Copy(ioutil.Discard, response)
		if err != nil {
			log.Fatal("failed to copy data", err)
		}
		if err = response.Close(); err != nil {
			return err
		}
	}

	for i, test := range tests {
		fmt.Println("Preparing for testcase", i)
		time.Sleep(sleep)

		rotator.Reset(maxPaths)
		static.Reset()

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
		if err = response.Close(); err != nil {
			return err
		}

		test.duration += time.Since(start)

		// Can't compare functions in Go, thus we don't know which selector has been used
		// However, rotator will return 0 only if it has not been used, thus
		// a value of 0 will correspond to the other selector
		test.pathNumber = rotator.GetNumberOfUsedPaths()
	}

	fmt.Println()

	fmt.Println("--------------")

	writeToCsv(tests)

	return nil
}
