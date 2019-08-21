package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	ftp "github.com/elwin/transmit2/client"
)

func main() {

	if err := run(); err != nil {
		fmt.Print(err)
	}

}

func run() error {

	remote := "1-ff00:0:110,[127.0.0.1]:2121"
	local := "1-ff00:0:112,[127.0.0.1]:4000"

	conn, err := ftp.Dial(local, remote, ftp.DialWithDebugOutput(os.Stdout))
	if err != nil {
		return err
	}

	err = conn.Login("admin", "123456")
	if err != nil {
		return err
	}

	/*
		err = conn.Mode(mode.ExtendedBlockMode)
		if err != nil {
			return err
		}

	*/

	entries, err := conn.List("/")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fmt.Printf("- %s (%d)\n", entry.Name, entry.Size)
	}

	err = ReadAndWrite(conn)
	if err != nil {
		return err
	}

	return conn.Quit()
}

func ReadAndWrite(conn *ftp.ServerConn) error {
	err := conn.Stor("stor.txt", strings.NewReader("Hello World!"))
	if err != nil {
		return err
	}

	res, err := conn.Retr("retr.txt")

	f, err := os.Create("/home/elwin/ftp/result.txt")
	if err != nil {
		return err
	}

	n, err := io.Copy(f, res)
	if err != nil {
		return err
	}

	fmt.Printf("Read %d bytes\n", n)

	return res.Close()
}
