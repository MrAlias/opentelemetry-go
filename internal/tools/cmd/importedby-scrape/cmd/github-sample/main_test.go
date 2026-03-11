// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilterRows(t *testing.T) {
	rows := []repoRow{
		{FullName: "a/a", Stars: 10},
		{FullName: "a/b", Stars: 5, Fork: true},
		{FullName: "a/c", Stars: 7, Archived: true},
		{FullName: "a/d", Stars: 1},
	}
	got := filterRows(rows, config{excludeForks: true, excludeArchived: true, minStars: 2})
	if len(got) != 1 || got[0].FullName != "a/a" {
		t.Fatalf("filterRows() = %+v", got)
	}
}

func TestSampleRows(t *testing.T) {
	rows := []repoRow{
		{FullName: "a/1", Stars: 100},
		{FullName: "a/2", Stars: 90},
		{FullName: "a/3", Stars: 80},
		{FullName: "a/4", Stars: 70},
		{FullName: "a/5", Stars: 60},
		{FullName: "a/6", Stars: 50},
		{FullName: "a/7", Stars: 40},
		{FullName: "a/8", Stars: 30},
		{FullName: "a/9", Stars: 20},
	}
	got := sampleRows(rows, config{topN: 2, midN: 2, tailN: 2, seed: 1})
	if len(got) < 4 {
		t.Fatalf("sampleRows() length = %d, want at least 4", len(got))
	}
	if got[0].FullName != "a/1" || got[1].FullName != "a/2" {
		t.Fatalf("top rows not preserved: %+v", got[:2])
	}
}

func TestReadRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.csv")
	content := "full_name,stars,archived,fork,url\nacme/service,42,false,false,https://github.com/acme/service\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	rows, err := readRows(path)
	if err != nil {
		t.Fatalf("readRows: %v", err)
	}
	if len(rows) != 1 || rows[0].FullName != "acme/service" || rows[0].Stars != 42 {
		t.Fatalf("readRows() = %+v", rows)
	}
}
