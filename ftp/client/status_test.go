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

import "testing"

func TestValidStatusText(t *testing.T) {
	txt := StatusText(StatusInvalidCredentials)
	if txt == "" {
		t.Fatal("exptected status text, got empty string")
	}
}

func TestInvalidStatusText(t *testing.T) {
	txt := StatusText(0)
	if txt != "" {
		t.Fatalf("got status text %q, expected empty string", txt)
	}
}
