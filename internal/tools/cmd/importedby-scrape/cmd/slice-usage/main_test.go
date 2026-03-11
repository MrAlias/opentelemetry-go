// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestAttributeImportAliases(t *testing.T) {
	src := `package x
import attr "go.opentelemetry.io/otel/attribute"
var _ = attr.StringSlice("k", []string{"a"})
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	aliases := attributeImportAliases(file)
	if _, ok := aliases["attr"]; !ok {
		t.Fatalf("expected alias attr, got %#v", aliases)
	}
}

func TestScanFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	src := `package x
import "go.opentelemetry.io/otel/attribute"
func f(v []string) {
	_ = attribute.StringSlice("a", []string{"x", "y"})
	_ = attribute.StringSlice("b", v)
	_ = attribute.IntSlice("c", []int{1, 2, 3})
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	results := map[string]*stats{
		"StringSlice":  {literal: make(map[int]int)},
		"IntSlice":     {literal: make(map[int]int)},
		"BoolSlice":    {literal: make(map[int]int)},
		"Int64Slice":   {literal: make(map[int]int)},
		"Float64Slice": {literal: make(map[int]int)},
	}
	if err := scanFile(path, results); err != nil {
		t.Fatalf("scanFile: %v", err)
	}
	if results["StringSlice"].literal[2] != 1 || results["StringSlice"].dynamic != 1 {
		t.Fatalf("unexpected StringSlice stats: %+v", results["StringSlice"])
	}
	if results["IntSlice"].literal[3] != 1 {
		t.Fatalf("unexpected IntSlice stats: %+v", results["IntSlice"])
	}
}
