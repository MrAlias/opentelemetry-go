// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	cfg := config{}
	flag.StringVar(&cfg.inPath, "in", "", "Input file with one GitHub repo URL per line")
	flag.StringVar(&cfg.outDir, "out-dir", "", "Directory to clone repositories into")
	flag.IntVar(&cfg.depth, "depth", 1, "Clone depth")
	flag.BoolVar(&cfg.resume, "resume", true, "Skip repositories that already exist locally")
	flag.Parse()

	if err := run(cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "github-clone: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	inPath string
	outDir string
	depth  int
	resume bool
}

func run(cfg config) error {
	if cfg.inPath == "" {
		return errors.New("input path is required")
	}
	if cfg.outDir == "" {
		return errors.New("output directory is required")
	}
	if cfg.depth < 1 {
		return errors.New("depth must be at least 1")
	}
	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	repos, err := readRepoURLs(cfg.inPath)
	if err != nil {
		return err
	}

	var cloned, skipped int
	for _, repo := range repos {
		dst, err := clonePath(cfg.outDir, repo)
		if err != nil {
			return err
		}
		if cfg.resume {
			if _, err := os.Stat(filepath.Join(dst, ".git")); err == nil {
				skipped++
				continue
			}
		}
		if err := cloneRepo(repo, dst, cfg.depth); err != nil {
			return err
		}
		cloned++
	}

	fmt.Fprintf(os.Stderr, "github-clone: cloned=%d skipped=%d total=%d\n", cloned, skipped, len(repos))
	return nil
}

func readRepoURLs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open input file: %w", err)
	}
	defer f.Close()

	var repos []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		repos = append(repos, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan input file: %w", err)
	}
	return repos, nil
}

func clonePath(root, repoURL string) (string, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(repoURL), ".git")
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid repo URL %q", repoURL)
	}
	return filepath.Join(root, parts[0], parts[1], parts[2]), nil
}

func cloneRepo(repoURL, dst string, depth int) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	cmd := exec.Command("git", "clone", "--depth", fmt.Sprintf("%d", depth), repoURL, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone %s: %w: %s", repoURL, err, strings.TrimSpace(string(out)))
	}
	return nil
}
