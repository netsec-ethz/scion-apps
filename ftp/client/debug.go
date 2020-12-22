// Copyright 2020 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ftp

import "io"

type debugWrapper struct {
	conn io.ReadWriteCloser
	io.Reader
	io.Writer
}

func newDebugWrapper(conn io.ReadWriteCloser, w io.Writer) io.ReadWriteCloser {
	return &debugWrapper{
		Reader: io.TeeReader(conn, w),
		Writer: io.MultiWriter(w, conn),
		conn:   conn,
	}
}

func (w *debugWrapper) Close() error {
	return w.conn.Close()
}
