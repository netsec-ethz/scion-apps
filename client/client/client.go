package main

import (
	"bytes"
	"fmt"
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
	conn, err := ftp.Dial("localhost:2121", ftp.DialWithDebugOutput(os.Stdout))
	if err != nil {
		return err
	}

	err = conn.Login("admin", "123456")
	if err != nil {
		return err
	}

	err = conn.Stor("stor.txt", strings.NewReader("Hello World!"))
	if err != nil {
		return err
	}

	res, err := conn.Retr("stor.txt")

	buf := new(bytes.Buffer)
	buf.ReadFrom(res)
	if err != nil {
		return err
	}
	fmt.Printf("- %s\n", buf)
	res.Close()

	entries, err := conn.List("/")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fmt.Printf("- %s (%d)\n", entry.Name, entry.Size)
	}

	return nil
}
