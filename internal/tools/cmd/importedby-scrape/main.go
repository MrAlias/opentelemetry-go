// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const defaultBaseURL = "https://pkg.go.dev"

func main() {
	cfg := config{}
	flag.StringVar(&cfg.pkg, "package", "go.opentelemetry.io/otel/attribute", "Go package path to inspect on pkg.go.dev")
	flag.StringVar(&cfg.baseURL, "base-url", defaultBaseURL, "Base URL for pkg.go.dev-compatible site")
	flag.StringVar(&cfg.htmlPath, "html", "", "Optional path to a saved imported-by HTML page")
	flag.StringVar(&cfg.outPath, "out", "", "Optional output file for discovered importer package paths")
	flag.DurationVar(&cfg.timeout, "timeout", 20*time.Second, "HTTP timeout")
	flag.Parse()

	importers, err := run(cfg)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "importedby-scrape: %v\n", err)
		os.Exit(1)
	}

	w := io.Writer(os.Stdout)
	if cfg.outPath != "" {
		f, createErr := os.Create(cfg.outPath)
		if createErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "importedby-scrape: create output: %v\n", createErr)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}

	for _, importer := range importers {
		_, _ = fmt.Fprintln(w, importer)
	}
}

type config struct {
	pkg      string
	baseURL  string
	htmlPath string
	outPath  string
	timeout  time.Duration
}

func run(cfg config) ([]string, error) {
	if cfg.pkg == "" {
		return nil, errors.New("package path is required")
	}

	var doc *goquery.Document
	var err error
	if cfg.htmlPath != "" {
		doc, err = loadDocumentFile(cfg.htmlPath)
	} else {
		doc, err = loadDocumentURL(importedByURL(cfg.baseURL, cfg.pkg), cfg.timeout)
	}
	if err != nil {
		return nil, err
	}

	importers := extractImporters(doc, cfg.pkg)
	if len(importers) == 0 {
		return nil, errors.New("no importer package paths found")
	}
	return importers, nil
}

func importedByURL(baseURL, pkg string) string {
	base := strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/%s?tab=importedby", base, pkg)
}

func loadDocumentFile(filePath string) (*goquery.Document, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open html file: %w", err)
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		return nil, fmt.Errorf("parse html file: %w", err)
	}
	return doc, nil
}

func loadDocumentURL(rawURL string, timeout time.Duration) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		rawURL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "otel-importedby-scrape/0.1")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch imported-by page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch imported-by page: status %s", resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse imported-by page: %w", err)
	}
	return doc, nil
}

func extractImporters(doc *goquery.Document, targetPkg string) []string {
	seen := make(map[string]struct{})

	doc.Find("main a[href]").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}

		pkgPath, ok := importerPathFromHref(href)
		if !ok || pkgPath == targetPkg {
			return
		}

		text := strings.TrimSpace(s.Text())
		if text != "" && text != pkgPath {
			return
		}

		seen[pkgPath] = struct{}{}
	})

	importers := make([]string, 0, len(seen))
	for importer := range seen {
		importers = append(importers, importer)
	}
	sort.Strings(importers)
	return importers
}

func importerPathFromHref(href string) (string, bool) {
	u, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	if u.Host != "" && u.Host != "pkg.go.dev" {
		return "", false
	}

	cleanPath := strings.TrimSpace(strings.TrimPrefix(path.Clean(u.Path), "/"))
	if cleanPath == "." || cleanPath == "" {
		return "", false
	}
	if strings.HasPrefix(cleanPath, "search") || strings.HasPrefix(cleanPath, "mod/") {
		return "", false
	}
	if strings.Contains(cleanPath, "@") {
		return "", false
	}
	if u.RawQuery != "" {
		return "", false
	}
	if strings.Count(cleanPath, "/") < 1 || !strings.Contains(cleanPath, ".") {
		return "", false
	}
	return cleanPath, true
}
