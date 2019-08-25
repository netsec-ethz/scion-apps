package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/elwin/transmit2/scion"

	"github.com/elwin/transmit/mode"
	ftp "github.com/elwin/transmit2/client"
)

var (
	local  = flag.String("local", "1-ff00:0:112,[127.0.0.1]:5000", "Host")
	remote = flag.String("remote", "1-ff00:0:110,[127.0.0.1]:2121", "Host")
)

func main() {

	if err := run(); err != nil {
		fmt.Println(err)
	}

}

type test struct {
	mode          byte
	parallelism   int
	payload       int // in MB
	blockSize     int
	selector      scion.PathSelector
	duration      time.Duration
	numberOfPaths int
}

func (test *test) String() string {
	if test.mode == mode.Stream {
		return fmt.Sprintf("Stream (paths: %d) with %d MB: %s", test.numberOfPaths, test.payload, test.duration)
	} else {
		return fmt.Sprintf("Extended (paths: %d, streams: %d, bs: %d) with %d MB: %s", test.numberOfPaths, test.parallelism, test.blockSize, test.payload, test.duration)
	}
}

func run() error {

	parallelisms := []int{1, 2, 4}
	// parallelisms := []int{8, 16, 32}
	payloads := []int{1}
	// blocksizes := []int{16384}
	blocksizes := []int{4096, 8192}
	// blocksizes := []int{512, 1024, 2048, 4096, 8192}

	rotator := scion.NewRotator()
	selectors := []scion.PathSelector{scion.DefaultPathSelector, rotator.RotatingSelector}

	var tests []*test
	for _, payload := range payloads {
		for _, selector := range selectors {
			t := &test{
				mode:     mode.Stream,
				selector: selector,
				payload:  payload,
			}
			tests = append(tests, t)

			for _, blocksize := range blocksizes {
				for _, parallelism := range parallelisms {
					t := &test{
						mode:        mode.ExtendedBlockMode,
						parallelism: parallelism,
						payload:     payload,
						selector:    selector,
						blockSize:   blocksize,
					}
					tests = append(tests, t)
				}
			}
		}
	}

	conn, err := ftp.Dial(*local, *remote)
	if err != nil {
		return err
	}

	if err = conn.Login("admin", "123456"); err != nil {
		return err
	}

	for _, test := range tests {
		conn.SetPathSelector(test.selector)
		rotator.Reset()

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
		response, err := conn.Retr(strconv.Itoa(test.payload * 1024 * 1024))
		if err != nil {
			return err
		}

		n, err := io.Copy(ioutil.Discard, response)
		if err != nil {
			return err
		}
		if int(n) != test.payload*1024*1024 {
			return fmt.Errorf("failed to read correct number of bytes, expected %d but got %d", test.payload*1024*1024, n)
		}
		response.Close()

		test.duration += time.Since(start)
		test.numberOfPaths = rotator.GetNumberOfUsedPaths()

		fmt.Print(".")
	}
	fmt.Println()
	conn.Quit()

	for _, test := range tests {
		fmt.Println(test)
	}

	return nil
}
