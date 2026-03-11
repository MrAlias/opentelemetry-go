// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	cfg := config{}
	flag.StringVar(&cfg.inPath, "in", "", "Input file with one importer package path per line")
	flag.StringVar(&cfg.outDir, "out-dir", "", "Optional output directory for categorized repo lists")
	flag.Parse()

	if err := run(cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "repo-candidates: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	inPath string
	outDir string
}

func run(cfg config) error {
	if cfg.inPath == "" {
		return errors.New("input file is required")
	}

	pkgs, err := readPackagePaths(cfg.inPath)
	if err != nil {
		return err
	}

	cats := categorize(pkgs)
	if cfg.outDir == "" {
		return writeSummary(os.Stdout, cats)
	}

	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	files := map[string][]string{
		"all.txt":              cats.all,
		"github.txt":           cats.github,
		"gitlab.txt":           cats.gitlab,
		"bitbucket.txt":        cats.bitbucket,
		"unknown-hosts.txt":    cats.unknownHosts,
		"unknown-packages.txt": cats.unknownPackages,
	}
	for name, values := range files {
		if err := writeLines(filepath.Join(cfg.outDir, name), values); err != nil {
			return err
		}
	}

	return writeSummary(os.Stdout, cats)
}

type categories struct {
	all             []string
	github          []string
	gitlab          []string
	bitbucket       []string
	unknownHosts    []string
	unknownPackages []string
}

func categorize(pkgs []string) categories {
	all := make(map[string]struct{})
	github := make(map[string]struct{})
	gitlab := make(map[string]struct{})
	bitbucket := make(map[string]struct{})
	unknownHosts := make(map[string]struct{})
	unknownPackages := make(map[string]struct{})

	for _, pkg := range pkgs {
		repo, hostKind, ok := repoCandidate(pkg)
		if !ok {
			unknownPackages[pkg] = struct{}{}
			continue
		}

		all[repo] = struct{}{}
		switch hostKind {
		case "github":
			github[repo] = struct{}{}
		case "gitlab":
			gitlab[repo] = struct{}{}
		case "bitbucket":
			bitbucket[repo] = struct{}{}
		default:
			unknownHosts[repo] = struct{}{}
		}
	}

	return categories{
		all:             mapKeysSorted(all),
		github:          mapKeysSorted(github),
		gitlab:          mapKeysSorted(gitlab),
		bitbucket:       mapKeysSorted(bitbucket),
		unknownHosts:    mapKeysSorted(unknownHosts),
		unknownPackages: mapKeysSorted(unknownPackages),
	}
}

func repoCandidate(pkg string) (repo string, hostKind string, ok bool) {
	parts := strings.Split(strings.TrimSpace(pkg), "/")
	if len(parts) < 3 || !strings.Contains(parts[0], ".") {
		return "", "", false
	}

	host := parts[0]
	switch host {
	case "github.com":
		return strings.Join(parts[:3], "/"), "github", true
	case "gitlab.com":
		return strings.Join(parts[:3], "/"), "gitlab", true
	case "bitbucket.org":
		return strings.Join(parts[:3], "/"), "bitbucket", true
	default:
		return strings.Join(parts[:3], "/"), "unknown", true
	}
}

func readPackagePaths(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open input file: %w", err)
	}
	defer f.Close()

	var pkgs []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		pkgs = append(pkgs, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan input file: %w", err)
	}
	return pkgs, nil
}

func writeSummary(w io.Writer, cats categories) error {
	_, err := fmt.Fprintf(
		w,
		"all=%d github=%d gitlab=%d bitbucket=%d unknown_hosts=%d unknown_packages=%d\n",
		len(cats.all),
		len(cats.github),
		len(cats.gitlab),
		len(cats.bitbucket),
		len(cats.unknownHosts),
		len(cats.unknownPackages),
	)
	return err
}

func writeLines(path string, lines []string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	for _, line := range lines {
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

func mapKeysSorted(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
