// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRepoCandidate(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		wantRepo string
		wantKind string
		wantOK   bool
	}{
		{name: "github", pkg: "github.com/acme/service/pkg/trace", wantRepo: "github.com/acme/service", wantKind: "github", wantOK: true},
		{name: "gitlab", pkg: "gitlab.com/acme/service/pkg/trace", wantRepo: "gitlab.com/acme/service", wantKind: "gitlab", wantOK: true},
		{name: "bitbucket", pkg: "bitbucket.org/acme/service/pkg/trace", wantRepo: "bitbucket.org/acme/service", wantKind: "bitbucket", wantOK: true},
		{name: "custom host", pkg: "example.com/acme/service/pkg/trace", wantRepo: "example.com/acme/service", wantKind: "unknown", wantOK: true},
		{name: "invalid", pkg: "stdlib/context", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRepo, gotKind, gotOK := repoCandidate(tt.pkg)
			if gotRepo != tt.wantRepo || gotKind != tt.wantKind || gotOK != tt.wantOK {
				t.Fatalf("repoCandidate(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.pkg, gotRepo, gotKind, gotOK, tt.wantRepo, tt.wantKind, tt.wantOK)
			}
		})
	}
}

func TestCategorize(t *testing.T) {
	got := categorize([]string{
		"github.com/acme/service/pkg/trace",
		"github.com/acme/service/pkg/http",
		"gitlab.com/acme/platform/internal/otel",
		"bitbucket.org/acme/team/pkg",
		"example.com/acme/service/pkg",
		"stdlib/context",
	})

	if diff := cmp.Diff([]string{
		"bitbucket.org/acme/team",
		"example.com/acme/service",
		"github.com/acme/service",
		"gitlab.com/acme/platform",
	}, got.all); diff != "" {
		t.Fatalf("all mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"github.com/acme/service"}, got.github); diff != "" {
		t.Fatalf("github mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"example.com/acme/service"}, got.unknownHosts); diff != "" {
		t.Fatalf("unknownHosts mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"stdlib/context"}, got.unknownPackages); diff != "" {
		t.Fatalf("unknownPackages mismatch (-want +got):\n%s", diff)
	}
}

func TestRunWritesOutputs(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "importers.txt")
	err := os.WriteFile(inPath, []byte(strings.Join([]string{
		"github.com/acme/service/pkg/trace",
		"gitlab.com/acme/platform/internal/otel",
		"",
	}, "\n")), 0o644)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	outDir := filepath.Join(dir, "out")
	if err := run(config{inPath: inPath, outDir: outDir}); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, name := range []string{
		"all.txt",
		"github.txt",
		"gitlab.txt",
		"bitbucket.txt",
		"unknown-hosts.txt",
		"unknown-packages.txt",
	} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatalf("expected output file %s: %v", name, err)
		}
	}
}
