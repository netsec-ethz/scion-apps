// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
//
// Copyright 2020-2021 ETH Zurich modifications to add support for SCION

package core

import (
	"fmt"
	"io"
)

var ErrNoFileSystem = fmt.Errorf("driver is not backed by a filesystem")

// DriverFactory is a driver factory to create driver. For each scionftp that connects to the server, a new FTPDriver is required.
// Create an implementation if this interface and provide it to FTPServer.
type DriverFactory interface {
	NewDriver() (Driver, error)
}

// Driver is an interface that you will create an implementation that speaks to your
// chosen persistence layer. graval will create a new instance of your
// driver for each scionftp that connects and delegate to it as required.
type Driver interface {
	// Init init
	Init(*Conn)

	// params  - a file path
	// returns - a time indicating when the requested path was last modified
	//         - an error if the file doesn't exist or the user lacks
	//           permissions
	Stat(string) (FileInfo, error)

	// params  - path
	// returns - true if the current user is permitted to change to the
	//           requested path
	ChangeDir(string) error

	// params  - path, function on file or subdir found
	// returns - error
	//           path
	ListDir(string, func(FileInfo) error) error

	// params  - path
	// returns - nil if the directory was deleted or any error encountered
	DeleteDir(string) error

	// params  - path
	// returns - nil if the file was deleted or any error encountered
	DeleteFile(string) error

	// params  - from_path, to_path
	// returns - nil if the file was renamed or any error encountered
	Rename(string, string) error

	// params  - path
	// returns - nil if the new directory was created or any error encountered
	MakeDir(string) error

	// params  - path
	// returns - a string containing the file data to send to the scionftp
	GetFile(string, int64) (int64, io.ReadCloser, error)

	// params  - destination path, an io.Reader containing the file data
	// returns - the number of bytes written and the first error encountered while writing, if any.
	PutFile(string, io.Reader, bool) (int64, error)

	// This function is only required to support the Hercules subsystem. If the driver is not backed by a filesystem,
	// this function should return a dummy value and IsFileSystem() must return false.
	// params  - path
	// returns - valid path in the filesystem to use with the Hercules subsystem
	RealPath(path string) string

	// returns - whether the driver is backed by a filesystem
	IsFileSystem() bool
}
