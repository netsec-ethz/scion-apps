package filedriver

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/netsec-ethz/scion-apps/ftp/server"
)

var _ server.Driver = &MockDriver{}

type MockDriver struct {
}

func (driver *MockDriver) Init(*server.Conn) {
	//
}

func (driver *MockDriver) Stat(string) (server.FileInfo, error) {
	panic("not impemented for mock driver")
}

func (driver *MockDriver) ChangeDir(string) error {
	panic("not impemented for mock driver")
}

func (driver *MockDriver) ListDir(string, func(server.FileInfo) error) error {
	panic("not impemented for mock driver")
}

func (driver *MockDriver) DeleteDir(string) error {
	panic("not impemented for mock driver")
}

func (driver *MockDriver) DeleteFile(string) error {
	panic("not impemented for mock driver")
}

func (driver *MockDriver) Rename(string, string) error {
	panic("not impemented for mock driver")
}

func (driver *MockDriver) MakeDir(string) error {
	panic("not impemented for mock driver")
}

func (driver *MockDriver) GetFile(path string, offset int64) (int64, io.ReadCloser, error) {
	path = strings.TrimLeft(path, "/")
	bytes, err := strconv.Atoi(path)
	if err != nil {
		return 0, nil, fmt.Errorf("path name must be length of file in bytes")
	}

	return int64(bytes), &RandomWriter{bytes}, nil
}

var _ io.ReadCloser = &RandomWriter{}

type RandomWriter struct {
	remainder int
}

func (writer *RandomWriter) Read(p []byte) (n int, err error) {
	if writer.remainder == 0 {
		return 0, io.EOF
	}

	n = len(p)
	if n > writer.remainder {
		n = writer.remainder
	}

	n, err = rand.Read(p[0:n])
	if err != nil {
		return n, err
	}

	writer.remainder -= n
	return n, nil
}

func (writer *RandomWriter) Close() error {
	return nil
}

func (driver *MockDriver) PutFile(path string, data io.Reader, appendData bool) (int64, error) {
	return io.Copy(ioutil.Discard, data)
}

var _ server.DriverFactory = &MockDriverFactory{}

type MockDriverFactory struct{}

func (factory *MockDriverFactory) NewDriver() (server.Driver, error) {
	return &MockDriver{}, nil
}
