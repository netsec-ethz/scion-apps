package main

import (
	"fmt"
	ftp "github.com/elwin/transmit2/client"
	"strings"
)

func main() {

	if err := run(); err != nil {
		fmt.Print(err)
	}

}

func run() error {
	conn, err := ftp.Dial("localhost:2121")
	if err != nil {
		return err
	}

	err = conn.Login("admin", "123456")
	if err != nil {
		return err
	}

	err = conn.Stor("stor.txt", strings.NewReader("Hello World"))
	if err != nil {
		return err
	}

	return nil
}
