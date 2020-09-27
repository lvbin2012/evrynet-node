// Copyright 2017 The evrynet-node Authors
// This file is part of the evrynet-node library.
//
// The evrynet-node library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The evrynet-node library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the evrynet-node library. If not, see <http://www.gnu.org/licenses/>.

package accounts

import (
	"testing"
)

func TestURLParsing(t *testing.T) {
	url, err := parseURL("https://Evrynetlabs.org")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if url.Scheme != "https" {
		t.Errorf("expected: %v, got: %v", "https", url.Scheme)
	}
	if url.Path != "Evrynetlabs.org" {
		t.Errorf("expected: %v, got: %v", "Evrynetlabs.org", url.Path)
	}

	_, err = parseURL("Evrynetlabs.org")
	if err == nil {
		t.Error("expected err, got: nil")
	}
}

func TestURLString(t *testing.T) {
	url := URL{Scheme: "https", Path: "Evrynetlabs.org"}
	if url.String() != "https://Evrynetlabs.org" {
		t.Errorf("expected: %v, got: %v", "https://Evrynetlabs.org", url.String())
	}

	url = URL{Scheme: "", Path: "Evrynetlabs.org"}
	if url.String() != "Evrynetlabs.org" {
		t.Errorf("expected: %v, got: %v", "Evrynetlabs.org", url.String())
	}
}

func TestURLMarshalJSON(t *testing.T) {
	url := URL{Scheme: "https", Path: "Evrynetlabs.org"}
	json, err := url.MarshalJSON()
	if err != nil {
		t.Errorf("unexpcted error: %v", err)
	}
	if string(json) != "\"https://Evrynetlabs.org\"" {
		t.Errorf("expected: %v, got: %v", "\"https://Evrynetlabs.org\"", string(json))
	}
}

func TestURLUnmarshalJSON(t *testing.T) {
	url := &URL{}
	err := url.UnmarshalJSON([]byte("\"https://Evrynetlabs.org\""))
	if err != nil {
		t.Errorf("unexpcted error: %v", err)
	}
	if url.Scheme != "https" {
		t.Errorf("expected: %v, got: %v", "https", url.Scheme)
	}
	if url.Path != "Evrynetlabs.org" {
		t.Errorf("expected: %v, got: %v", "https", url.Path)
	}
}

func TestURLComparison(t *testing.T) {
	tests := []struct {
		urlA   URL
		urlB   URL
		expect int
	}{
		{URL{"https", "Evrynetlabs.org"}, URL{"https", "Evrynetlabs.org"}, 0},
		{URL{"http", "Evrynetlabs.org"}, URL{"https", "Evrynetlabs.org"}, -1},
		{URL{"https", "Evrynetlabs.org/a"}, URL{"https", "Evrynetlabs.org"}, 1},
		{URL{"https", "abc.org"}, URL{"https", "Evrynetlabs.org"}, 1},
	}

	for i, tt := range tests {
		result := tt.urlA.Cmp(tt.urlB)
		if result != tt.expect {
			t.Errorf("test %d: cmp mismatch: expected: %d, got: %d", i, tt.expect, result)
		}
	}
}
