// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestClonePath(t *testing.T) {
	got, err := clonePath("/tmp/repos", "https://github.com/acme/service")
	if err != nil {
		t.Fatalf("clonePath: %v", err)
	}
	want := "/tmp/repos/github.com/acme/service"
	if got != want {
		t.Fatalf("clonePath() = %q, want %q", got, want)
	}
}
