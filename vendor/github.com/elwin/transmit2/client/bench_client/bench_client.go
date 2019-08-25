package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"time"

	ftp "github.com/elwin/transmit2/client"
)

var (
	local  = flag.String("local", "1-ff00:0:112,[127.0.0.1]:4000", "Host")
	remote = flag.String("remote", "1-ff00:0:110,[127.0.0.1]:2121", "Host")
)

func main() {

	flag.Parse()

	if err := run(); err != nil {
		fmt.Println(err)
	}

}

func run() error {

	conn, err := ftp.Dial(*local, *remote)
	if err != nil {
		return err
	}

	if err = conn.Login("admin", "123456"); err != nil {
		return err
	}

	/*
		err = conn.Mode(mode.ExtendedBlockMode)
		if err != nil {
			return err
		}
	*/

	tests := []struct {
		parallelism int
		duration    time.Duration
	}{
		{parallelism: 1},
		{parallelism: 2},
		{parallelism: 4},
		{parallelism: 8},
		{parallelism: 16},
		{parallelism: 32},
	}

	for j := 0; j < 5; j++ {
		for i := range tests {

			err = conn.SetRetrOpts(tests[i].parallelism, 500)
			if err != nil {
				return err
			}

			size := 1024 * 1024 // 1 MB

			start := time.Now()
			resp, err := conn.Retr(strconv.Itoa(size))
			if err != nil {
				return err
			}

			n, err := io.Copy(ioutil.Discard, resp)
			if err != nil {
				return err
			}
			if int(n) != size {
				return fmt.Errorf("failed to read correct number of bytes, expected %d but got %d", size, n)
			}
			resp.Close()

			tests[i].duration += time.Since(start)
			fmt.Print(".")
		}
		fmt.Println()

	}

	for i := range tests {
		fmt.Println(tests[i].duration)
	}

	return nil
}
