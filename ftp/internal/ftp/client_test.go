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
//
// Copyright 2020-2021 ETH Zurich modifications to add support for SCION

package ftp

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

const (
	testData = "Just some text"
	testDir  = "mydir"
)

func TestConnPASV(t *testing.T) {
	testConn(t, true)
}

func TestConnEPSV(t *testing.T) {
	testConn(t, false)
}

func testConn(t *testing.T, disableEPSV bool) {

	mock, c := openConn(t, "127.0.0.1", DialWithTimeout(5*time.Second), DialWithDisabledEPSV(disableEPSV))

	err := c.Login("anonymous", "anonymous")
	if err != nil {
		t.Fatal(err)
	}

	err = c.NoOp()
	if err != nil {
		t.Error(err)
	}

	err = c.ChangeDir("incoming")
	if err != nil {
		t.Error(err)
	}

	dir, err := c.CurrentDir()
	if err != nil {
		t.Error(err)
	} else {
		if dir != "/incoming" {
			t.Error("Wrong dir: " + dir)
		}
	}

	data := bytes.NewBufferString(testData)
	err = c.Stor("test", data)
	if err != nil {
		t.Error(err)
	}

	_, err = c.List(".")
	if err != nil {
		t.Error(err)
	}

	err = c.Rename("test", "tset")
	if err != nil {
		t.Error(err)
	}

	// Read without deadline
	r, err := c.Retr("tset")
	if err != nil {
		t.Error(err)
	} else {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			t.Error(err)
		}
		if string(buf) != testData {
			t.Errorf("'%s'", buf)
		}
		assert.NoError(t, r.Close())
		assert.NoError(t, r.Close()) // test we can close two times
	}

	// Read with deadline
	r, err = c.Retr("tset")
	if err != nil {
		t.Error(err)
	} else {
		assert.NoError(t, r.SetDeadline(time.Now()))
		_, err := ioutil.ReadAll(r)
		if err == nil {
			t.Error("deadline should have caused error")
		} else if !strings.HasSuffix(err.Error(), "i/o timeout") {
			t.Error(err)
		}
		assert.NoError(t, r.Close())
	}

	// Read with offset
	r, err = c.RetrFrom("tset", 5)
	if err != nil {
		t.Error(err)
	} else {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			t.Error(err)
		}
		expected := testData[5:]
		if string(buf) != expected {
			t.Errorf("read %q, expected %q", buf, expected)
		}
		assert.NoError(t, r.Close())
	}

	fileSize, err := c.FileSize("magic-file")
	if err != nil {
		t.Error(err)
	}
	if fileSize != 42 {
		t.Errorf("file size %q, expected %q", fileSize, 42)
	}

	_, err = c.FileSize("not-found")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	err = c.Delete("tset")
	if err != nil {
		t.Error(err)
	}

	err = c.MakeDir(testDir)
	if err != nil {
		t.Error(err)
	}

	err = c.ChangeDir(testDir)
	if err != nil {
		t.Error(err)
	}

	err = c.ChangeDirToParent()
	if err != nil {
		t.Error(err)
	}

	entries, err := c.NameList("/")
	if err != nil {
		t.Error(err)
	}
	if len(entries) != 1 || entries[0] != "/incoming" {
		t.Errorf("Unexpected entries: %v", entries)
	}

	err = c.RemoveDir(testDir)
	if err != nil {
		t.Error(err)
	}

	err = c.Logout()
	if err != nil {
		if protoErr := err.(*textproto.Error); protoErr != nil {
			if protoErr.Code != StatusNotImplemented {
				t.Error(err)
			}
		} else {
			t.Error(err)
		}
	}

	if err := c.Quit(); err != nil {
		t.Fatal(err)
	}

	// Wait for the connection to close
	mock.Wait()

	err = c.NoOp()
	if err == nil {
		t.Error("Expected error")
	}
}

func TestTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	c, err := DialTimeout("localhost:2121", 1*time.Second)
	if err == nil {
		t.Fatal("expected timeout, got nil error")
		_ = c.Quit()
	}
}

func TestWrongLogin(t *testing.T) {
	mock, err := newFtpMock(t, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	c, err := DialTimeout(mock.Addr(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, c.Quit()) }()

	err = c.Login("zoo2Shia", "fei5Yix9")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDeleteDirRecur(t *testing.T) {
	mock, c := openConn(t, "127.0.0.1")

	err := c.RemoveDirRecur("testDir")
	if err != nil {
		t.Error(err)
	}

	if err := c.Quit(); err != nil {
		t.Fatal(err)
	}

	// Wait for the connection to close
	mock.Wait()
}

// func TestFileDeleteDirRecur(t *testing.T) {
// 	mock, c := openConn(t, "127.0.0.1")

// 	err := c.RemoveDirRecur("testFile")
// 	if err == nil {
// 		t.Fatal("expected error got nil")
// 	}

// 	if err := c.Quit(); err != nil {
// 		t.Fatal(err)
// 	}

// 	// Wait for the connection to close
// 	mock.Wait()
// }

func TestMissingFolderDeleteDirRecur(t *testing.T) {
	mock, c := openConn(t, "127.0.0.1")

	err := c.RemoveDirRecur("missing-dir")
	if err == nil {
		t.Fatal("expected error got nil")
	}

	if err := c.Quit(); err != nil {
		t.Fatal(err)
	}

	// Wait for the connection to close
	mock.Wait()
}
