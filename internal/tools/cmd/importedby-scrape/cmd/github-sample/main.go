// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
)

func main() {
	cfg := config{}
	flag.StringVar(&cfg.inPath, "in", "", "Input metadata CSV from github-metadata")
	flag.StringVar(&cfg.outPath, "out", "", "Output file with one repo URL per line")
	flag.BoolVar(&cfg.excludeForks, "exclude-forks", true, "Exclude forked repositories")
	flag.BoolVar(&cfg.excludeArchived, "exclude-archived", true, "Exclude archived repositories")
	flag.IntVar(&cfg.minStars, "min-stars", 0, "Minimum star threshold")
	flag.IntVar(&cfg.topN, "top", 100, "Take the first N repos by stars after filtering")
	flag.IntVar(&cfg.midN, "mid", 100, "Take N repos from the middle star tier")
	flag.IntVar(&cfg.tailN, "tail", 100, "Take N random repos from the tail tier")
	flag.Int64Var(&cfg.seed, "seed", 42, "Random seed for tail sampling")
	flag.Parse()

	if err := run(cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "github-sample: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	inPath          string
	outPath         string
	excludeForks    bool
	excludeArchived bool
	minStars        int
	topN            int
	midN            int
	tailN           int
	seed            int64
}

type repoRow struct {
	FullName string
	Stars    int
	Archived bool
	Fork     bool
	URL      string
}

func run(cfg config) error {
	if cfg.inPath == "" {
		return errors.New("input path is required")
	}
	if cfg.outPath == "" {
		return errors.New("output path is required")
	}

	rows, err := readRows(cfg.inPath)
	if err != nil {
		return err
	}
	rows = filterRows(rows, cfg)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Stars != rows[j].Stars {
			return rows[i].Stars > rows[j].Stars
		}
		return rows[i].FullName < rows[j].FullName
	})

	sampled := sampleRows(rows, cfg)
	if err := writeURLs(cfg.outPath, sampled); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "github-sample: input=%d filtered=%d sampled=%d\n", len(rows), len(rows), len(sampled))
	return nil
}

func readRows(path string) ([]repoRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(records) < 1 {
		return nil, errors.New("empty csv")
	}

	header := make(map[string]int, len(records[0]))
	for i, name := range records[0] {
		header[name] = i
	}

	required := []string{"full_name", "stars", "archived", "fork", "url"}
	for _, name := range required {
		if _, ok := header[name]; !ok {
			return nil, fmt.Errorf("missing required column %q", name)
		}
	}

	rows := make([]repoRow, 0, len(records)-1)
	for _, record := range records[1:] {
		stars, err := strconv.Atoi(record[header["stars"]])
		if err != nil {
			return nil, fmt.Errorf("parse stars for %q: %w", record[header["full_name"]], err)
		}
		archived, err := strconv.ParseBool(record[header["archived"]])
		if err != nil {
			return nil, fmt.Errorf("parse archived for %q: %w", record[header["full_name"]], err)
		}
		fork, err := strconv.ParseBool(record[header["fork"]])
		if err != nil {
			return nil, fmt.Errorf("parse fork for %q: %w", record[header["full_name"]], err)
		}
		rows = append(rows, repoRow{
			FullName: record[header["full_name"]],
			Stars:    stars,
			Archived: archived,
			Fork:     fork,
			URL:      record[header["url"]],
		})
	}
	return rows, nil
}

func filterRows(rows []repoRow, cfg config) []repoRow {
	out := rows[:0]
	for _, row := range rows {
		if cfg.excludeForks && row.Fork {
			continue
		}
		if cfg.excludeArchived && row.Archived {
			continue
		}
		if row.Stars < cfg.minStars {
			continue
		}
		out = append(out, row)
	}
	return out
}

func sampleRows(rows []repoRow, cfg config) []repoRow {
	if len(rows) == 0 {
		return nil
	}

	selected := make(map[string]repoRow)
	addRows := func(in []repoRow) {
		for _, row := range in {
			selected[row.FullName] = row
		}
	}

	topEnd := min(cfg.topN, len(rows))
	addRows(rows[:topEnd])

	remaining := rows[topEnd:]
	midStart := len(remaining) / 3
	midEnd := min(midStart+cfg.midN, len(remaining))
	if midStart < len(remaining) {
		addRows(remaining[midStart:midEnd])
	}

	if len(remaining) > 0 && cfg.tailN > 0 {
		tailStart := (2 * len(remaining)) / 3
		tail := append([]repoRow(nil), remaining[tailStart:]...)
		rng := rand.New(rand.NewSource(cfg.seed))
		rng.Shuffle(len(tail), func(i, j int) {
			tail[i], tail[j] = tail[j], tail[i]
		})
		addRows(tail[:min(cfg.tailN, len(tail))])
	}

	out := make([]repoRow, 0, len(selected))
	for _, row := range selected {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Stars != out[j].Stars {
			return out[i].Stars > out[j].Stars
		}
		return out[i].FullName < out[j].FullName
	})
	return out
}

func writeURLs(path string, rows []repoRow) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer f.Close()

	for _, row := range rows {
		url := row.URL
		if url == "" {
			url = "https://github.com/" + strings.TrimPrefix(row.FullName, "github.com/")
		}
		if _, err := fmt.Fprintln(f, url); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
