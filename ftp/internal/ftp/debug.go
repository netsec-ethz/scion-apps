// Copyright (c) 2011-2013, Julien Laffaye <jlaffaye@FreeBSD.org>
//
// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
// WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
// ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
// WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
// ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
// OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

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
