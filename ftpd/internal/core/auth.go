// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
//
// Copyright 2021 ETH Zurich modifications to add anonymous authentication

package core

import (
	"crypto/subtle"
)

// Auth is an interface to auth your ftp user login.
type Auth interface {
	// Verifies login credentials
	CheckPasswd(string, string) (bool, error)
	// Checks if the supplied user is authorized to perform actions that require authentication
	IsAuthorized(string) bool
}

var (
	_ Auth = &SimpleAuth{}
)

// SimpleAuth implements Auth interface to provide a memory user login auth
type SimpleAuth struct {
	Name     string
	Password string
}

// CheckPasswd will check user's password
func (a *SimpleAuth) CheckPasswd(name, pass string) (bool, error) {
	return constantTimeEquals(name, a.Name) && constantTimeEquals(pass, a.Password), nil
}

func constantTimeEquals(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (a *SimpleAuth) IsAuthorized(s string) bool {
	return s != ""
}

// AnonymousAuth implements Auth interface to enable authentication commands when using anonymous FTP
type AnonymousAuth struct{}

func (a AnonymousAuth) CheckPasswd(name, pass string) (bool, error) {
	return name == "anonymous", nil
}

func (a AnonymousAuth) IsAuthorized(name string) bool {
	return true
}
