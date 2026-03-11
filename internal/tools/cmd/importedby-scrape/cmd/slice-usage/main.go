// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	cfg := config{}
	flag.StringVar(&cfg.rootDir, "root", "", "Root directory containing cloned repositories")
	flag.StringVar(&cfg.outPath, "out", "", "Output CSV path for per-constructor histogram")
	flag.Parse()

	if err := run(cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "slice-usage: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	rootDir string
	outPath string
}

type stats struct {
	literal map[int]int
	dynamic int
	total   int
}

var constructors = map[string]struct{}{
	"BoolSlice":    {},
	"IntSlice":     {},
	"Int64Slice":   {},
	"Float64Slice": {},
	"StringSlice":  {},
}

func run(cfg config) error {
	if cfg.rootDir == "" {
		return errors.New("root directory is required")
	}
	if cfg.outPath == "" {
		return errors.New("output path is required")
	}

	results := map[string]*stats{}
	for name := range constructors {
		results[name] = &stats{literal: make(map[int]int)}
	}

	err := filepath.WalkDir(cfg.rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "vendor" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		return scanFile(path, results)
	})
	if err != nil {
		return err
	}

	if err := writeResults(cfg.outPath, results); err != nil {
		return err
	}
	printSummary(results)
	return nil
}

func scanFile(path string, results map[string]*stats) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil
	}

	importAliases := attributeImportAliases(file)
	if len(importAliases) == 0 {
		return nil
	}

	globalEnv := make(lengthEnv)
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok != token.VAR {
				continue
			}
			processGenDecl(d, globalEnv, importAliases, results)
		case *ast.FuncDecl:
			processBlock(d.Body, globalEnv, importAliases, results)
		}
	}
	return nil
}

func attributeImportAliases(file *ast.File) map[string]struct{} {
	aliases := make(map[string]struct{})
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != "go.opentelemetry.io/otel/attribute" {
			continue
		}
		if imp.Name != nil {
			aliases[imp.Name.Name] = struct{}{}
		} else {
			aliases["attribute"] = struct{}{}
		}
	}
	return aliases
}

func literalSliceLen(expr ast.Expr) (int, bool) {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return 0, false
	}
	at, ok := lit.Type.(*ast.ArrayType)
	if !ok || at.Len != nil {
		return 0, false
	}
	return len(lit.Elts), true
}

type lengthEnv map[string]lengthValue

type lengthValue struct {
	length int
	known  bool
}

func (e lengthEnv) clone() lengthEnv {
	out := make(lengthEnv, len(e))
	for k, v := range e {
		out[k] = v
	}
	return out
}

func processBlock(block *ast.BlockStmt, base lengthEnv, importAliases map[string]struct{}, results map[string]*stats) {
	if block == nil {
		return
	}
	env := base.clone()
	for _, stmt := range block.List {
		processStmt(stmt, env, importAliases, results)
	}
}

func processStmt(stmt ast.Stmt, env lengthEnv, importAliases map[string]struct{}, results map[string]*stats) {
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		processBlock(s, env, importAliases, results)
	case *ast.ExprStmt:
		processExpr(s.X, env, importAliases, results)
	case *ast.AssignStmt:
		for _, rhs := range s.Rhs {
			processExpr(rhs, env, importAliases, results)
		}
		for i, lhs := range s.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok || ident.Name == "_" {
				continue
			}
			if i >= len(s.Rhs) {
				env[ident.Name] = lengthValue{}
				continue
			}
			env[ident.Name] = resolveLen(s.Rhs[i], env)
		}
	case *ast.DeclStmt:
		if d, ok := s.Decl.(*ast.GenDecl); ok {
			processGenDecl(d, env, importAliases, results)
		}
	case *ast.ReturnStmt:
		for _, expr := range s.Results {
			processExpr(expr, env, importAliases, results)
		}
	case *ast.IfStmt:
		if s.Init != nil {
			processStmt(s.Init, env, importAliases, results)
		}
		if s.Cond != nil {
			processExpr(s.Cond, env, importAliases, results)
		}
		processBlock(s.Body, env, importAliases, results)
		if s.Else != nil {
			processStmt(s.Else, env, importAliases, results)
		}
	case *ast.ForStmt:
		if s.Init != nil {
			processStmt(s.Init, env, importAliases, results)
		}
		if s.Cond != nil {
			processExpr(s.Cond, env, importAliases, results)
		}
		if s.Post != nil {
			processStmt(s.Post, env, importAliases, results)
		}
		processBlock(s.Body, env, importAliases, results)
	case *ast.RangeStmt:
		processExpr(s.X, env, importAliases, results)
		processBlock(s.Body, env, importAliases, results)
	case *ast.SwitchStmt:
		if s.Init != nil {
			processStmt(s.Init, env, importAliases, results)
		}
		if s.Tag != nil {
			processExpr(s.Tag, env, importAliases, results)
		}
		for _, stmt := range s.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			clauseEnv := env.clone()
			for _, expr := range clause.List {
				processExpr(expr, clauseEnv, importAliases, results)
			}
			for _, inner := range clause.Body {
				processStmt(inner, clauseEnv, importAliases, results)
			}
		}
	case *ast.TypeSwitchStmt:
		if s.Init != nil {
			processStmt(s.Init, env, importAliases, results)
		}
		processStmt(s.Assign, env, importAliases, results)
		for _, stmt := range s.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			clauseEnv := env.clone()
			for _, inner := range clause.Body {
				processStmt(inner, clauseEnv, importAliases, results)
			}
		}
	case *ast.SelectStmt:
		for _, stmt := range s.Body.List {
			clause, ok := stmt.(*ast.CommClause)
			if !ok {
				continue
			}
			clauseEnv := env.clone()
			if clause.Comm != nil {
				processStmt(clause.Comm, clauseEnv, importAliases, results)
			}
			for _, inner := range clause.Body {
				processStmt(inner, clauseEnv, importAliases, results)
			}
		}
	case *ast.GoStmt:
		processExpr(s.Call, env, importAliases, results)
	case *ast.DeferStmt:
		processExpr(s.Call, env, importAliases, results)
	case *ast.SendStmt:
		processExpr(s.Chan, env, importAliases, results)
		processExpr(s.Value, env, importAliases, results)
	}
}

func processGenDecl(d *ast.GenDecl, env lengthEnv, importAliases map[string]struct{}, results map[string]*stats) {
	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, value := range vs.Values {
			processExpr(value, env, importAliases, results)
		}
		for i, name := range vs.Names {
			if name.Name == "_" {
				continue
			}
			if i >= len(vs.Values) {
				env[name.Name] = lengthValue{}
				continue
			}
			env[name.Name] = resolveLen(vs.Values[i], env)
		}
	}
}

func processExpr(expr ast.Expr, env lengthEnv, importAliases map[string]struct{}, results map[string]*stats) {
	if expr == nil {
		return
	}
	if call, ok := expr.(*ast.CallExpr); ok {
		recordCall(call, env, importAliases, results)
	}
	ast.Inspect(expr, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || call == expr {
			return true
		}
		recordCall(call, env, importAliases, results)
		return true
	})
}

func recordCall(call *ast.CallExpr, env lengthEnv, importAliases map[string]struct{}, results map[string]*stats) {
	if len(call.Args) < 2 {
		return
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	if _, ok := importAliases[pkgIdent.Name]; !ok {
		return
	}
	if _, ok := constructors[sel.Sel.Name]; !ok {
		return
	}

	stat := results[sel.Sel.Name]
	stat.total++
	if v := resolveLen(call.Args[1], env); v.known {
		stat.literal[v.length]++
	} else {
		stat.dynamic++
	}
}

func resolveLen(expr ast.Expr, env lengthEnv) lengthValue {
	if n, ok := literalSliceLen(expr); ok {
		return lengthValue{length: n, known: true}
	}
	if ident, ok := expr.(*ast.Ident); ok {
		if v, ok := env[ident.Name]; ok {
			return v
		}
	}
	return lengthValue{}
}

func writeResults(path string, results map[string]*stats) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"constructor", "kind", "value", "count"}); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	names := make([]string, 0, len(results))
	for name := range results {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		stat := results[name]
		lens := make([]int, 0, len(stat.literal))
		for n := range stat.literal {
			lens = append(lens, n)
		}
		sort.Ints(lens)
		for _, n := range lens {
			if err := w.Write([]string{name, "literal_len", strconv.Itoa(n), strconv.Itoa(stat.literal[n])}); err != nil {
				return fmt.Errorf("write literal row: %w", err)
			}
		}
		for _, row := range [][2]string{
			{"dynamic", strconv.Itoa(stat.dynamic)},
			{"total", strconv.Itoa(stat.total)},
		} {
			if err := w.Write([]string{name, row[0], "", row[1]}); err != nil {
				return fmt.Errorf("write aggregate row: %w", err)
			}
		}
	}
	return w.Error()
}

func printSummary(results map[string]*stats) {
	names := make([]string, 0, len(results))
	for name := range results {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		stat := results[name]
		covered2 := countUpTo(stat.literal, 2)
		covered3 := countUpTo(stat.literal, 3)
		covered4 := countUpTo(stat.literal, 4)
		fmt.Fprintf(
			os.Stderr,
			"%s total=%d dynamic=%d literal<=2=%d literal<=3=%d literal<=4=%d\n",
			name, stat.total, stat.dynamic, covered2, covered3, covered4,
		)
	}
}

func countUpTo(m map[int]int, maxLen int) int {
	var total int
	for n, count := range m {
		if n <= maxLen {
			total += count
		}
	}
	return total
}
