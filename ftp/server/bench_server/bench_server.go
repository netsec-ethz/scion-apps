// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// This is a very simple ftpd server using this library as an example
// and as something to run tests against.
package main

import (
	"flag"
	"log"

	filedriver "github.com/netsec-ethz/scion-apps/ftp/file-driver"
	"github.com/netsec-ethz/scion-apps/ftp/server"
)

const (
	username = "admin"
	password = "123456"
)

func main() {
	var (
		port = flag.Int("port", 2121, "Port")
		host = flag.String("host", "", "Host")
	)
	flag.Parse()

	if *host == "" {
		log.Fatalf("Please set the hostaddress with -host")
	}

	factory := &filedriver.MockDriverFactory{}

	opts := &server.Opts{
		Factory:  factory,
		Port:     uint16(*port),
		Hostname: *host,
		Auth:     &server.SimpleAuth{Name: username, Password: password},
	}

	log.Printf("Starting ftp server on %v:%v", opts.Hostname, opts.Port)
	log.Printf("Username %v, Password %v", username, password)
	server := server.NewServer(opts)
	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("Error starting server:", err)
	}
}
