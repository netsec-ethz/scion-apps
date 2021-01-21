// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
//
// Modifications 2019 Elwin Stephan to make it compatible to SCION and
// introduce parts of the GridFTP extension
//
// Copyright 2020-2021 ETH Zurich modifications to add support for SCION
package main

import (
	"flag"
	"log"

	"github.com/netsec-ethz/scion-apps/ftpd/internal/core"
	driver "github.com/netsec-ethz/scion-apps/ftpd/internal/driver/file"
	hercules2 "github.com/netsec-ethz/scion-apps/internal/ftp/hercules"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
)

func main() {
	var (
		root     = flag.String("root", "", "Root directory to serve")
		user     = flag.String("user", "", "Username for login (omit for public access)")
		pass     = flag.String("pass", "", "Password for login (omit for public access)")
		port     = flag.Uint("port", 2121, "Port")
		hercules = flag.String("hercules", "", "Enable Hercules mode using the Hercules binary specified\nIn Hercules mode, scionFTP checks the following directories for Hercules config files: ., /etc, /etc/scion-ftp")
	)
	flag.Parse()
	if *root == "" {
		log.Fatalf("Please set a root to serve with -root")
	}

	factory := &driver.FileDriverFactory{
		RootPath: *root,
		Perm:     core.NewSimplePerm("user", "group"),
	}

	certs := appquic.GetDummyTLSCerts()

	var auth core.Auth
	if *user == "" && *pass == "" {
		log.Printf("Anonymous FTP")
		auth = &core.AnonymousAuth{}
	} else {
		log.Printf("Username %v, Password %v", *user, *pass)
		auth = &core.SimpleAuth{Name: *user, Password: *pass}
	}

	herculesConfig, err := hercules2.ResolveConfig()
	if err != nil {
		log.Printf("hercules.ResolveConfig: %s", err)
	}
	if herculesConfig != nil {
		log.Printf("In Hercules mode, using configuration at %s", *herculesConfig)
	}

	opts := &core.Opts{
		Factory:        factory,
		Port:           uint16(*port),
		Auth:           auth,
		Certificate:    &certs[0],
		HerculesBinary: *hercules,
		HerculesConfig: herculesConfig,
		RootPath:       *root,
	}

	srv := core.NewServer(opts)
	err = srv.ListenAndServe()
	if err != nil {
		log.Fatal("Error starting server:", err)
	}
}
