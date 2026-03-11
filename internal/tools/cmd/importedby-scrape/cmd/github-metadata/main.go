// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const githubGraphQLBaseURL = "https://api.github.com/graphql"

func main() {
	cfg := config{}
	flag.StringVar(&cfg.inPath, "in", "", "Input file with one github.com/org/repo per line")
	flag.StringVar(&cfg.outPath, "out", "", "Output CSV path")
	flag.StringVar(&cfg.token, "token", firstEnv("GITHUB_TOKEN", "GH_TOKEN"), "GitHub API token")
	flag.StringVar(&cfg.baseURL, "base-url", githubGraphQLBaseURL, "GitHub GraphQL API URL")
	flag.IntVar(&cfg.limit, "limit", 0, "Maximum number of repos to fetch (0 means all)")
	flag.IntVar(&cfg.concurrency, "concurrency", 4, "Number of concurrent GraphQL requests")
	flag.IntVar(&cfg.batchSize, "batch-size", 25, "Number of repos per GraphQL request")
	flag.DurationVar(&cfg.timeout, "timeout", 20*time.Second, "HTTP timeout")
	flag.Parse()

	if err := run(cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "github-metadata: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	inPath      string
	outPath     string
	token       string
	baseURL     string
	limit       int
	concurrency int
	batchSize   int
	timeout     time.Duration
}

type repoMetadata struct {
	FullName        string
	Stars           int
	Forks           int
	Archived        bool
	Fork            bool
	PushedAt        string
	DefaultBranch   string
	OpenIssuesCount int
	Language        string
	URL             string
}

type repoRef struct {
	Repo  string
	Owner string
	Name  string
}

type batchResult struct {
	metas   []repoMetadata
	missing []string
	errors  []string
}

type graphQLRequest struct {
	Query string `json:"query"`
}

type graphQLResponse struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []graphQLError             `json:"errors"`
}

type graphQLError struct {
	Message    string           `json:"message"`
	Path       []any            `json:"path"`
	Locations  []map[string]int `json:"locations"`
	Extensions map[string]any   `json:"extensions"`
}

type graphQLRepo struct {
	NameWithOwner    string `json:"nameWithOwner"`
	StargazerCount   int    `json:"stargazerCount"`
	ForkCount        int    `json:"forkCount"`
	IsArchived       bool   `json:"isArchived"`
	IsFork           bool   `json:"isFork"`
	PushedAt         string `json:"pushedAt"`
	URL              string `json:"url"`
	DefaultBranchRef *struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
	PrimaryLanguage *struct {
		Name string `json:"name"`
	} `json:"primaryLanguage"`
	Issues struct {
		TotalCount int `json:"totalCount"`
	} `json:"issues"`
}

func run(cfg config) error {
	if cfg.inPath == "" {
		return errors.New("input file is required")
	}
	if cfg.outPath == "" {
		return errors.New("output path is required")
	}
	if cfg.token == "" {
		return errors.New("GitHub token is required; set -token or GITHUB_TOKEN/GH_TOKEN")
	}
	if cfg.concurrency < 1 {
		return errors.New("concurrency must be at least 1")
	}
	if cfg.batchSize < 1 {
		return errors.New("batch-size must be at least 1")
	}

	repos, err := readRepoRefs(cfg.inPath)
	if err != nil {
		return err
	}

	progress, err := loadProgress(cfg.outPath)
	if err != nil {
		return err
	}
	repos = filterPending(repos, progress)
	if cfg.limit > 0 && cfg.limit < len(repos) {
		repos = repos[:cfg.limit]
	}

	writer, err := newOutputWriter(cfg.outPath)
	if err != nil {
		return err
	}
	defer writer.Close()

	client := &http.Client{Timeout: cfg.timeout}
	counts, err := processBatches(client, cfg, repos, writer)
	if err != nil {
		return err
	}

	fmt.Fprintf(
		os.Stderr,
		"github-metadata: resumed=%d wrote=%d missing=%d errors=%d remaining=0\n",
		progress.processedCount(),
		counts.wrote,
		counts.missing,
		counts.errors,
	)
	return nil
}

func readRepoRefs(path string) ([]repoRef, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open input file: %w", err)
	}
	defer f.Close()

	var repos []repoRef
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		ref, ok := parseRepoRef(line)
		if !ok {
			continue
		}
		repos = append(repos, ref)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan input file: %w", err)
	}
	return repos, nil
}

func parseRepoRef(s string) (repoRef, bool) {
	parts := strings.Split(strings.TrimSpace(s), "/")
	if len(parts) != 3 || parts[0] != "github.com" {
		return repoRef{}, false
	}
	return repoRef{
		Repo:  s,
		Owner: parts[1],
		Name:  parts[2],
	}, true
}

func processBatches(client httpClient, cfg config, repos []repoRef, writer *outputWriter) (resultCounts, error) {
	type workItem struct {
		batch []repoRef
	}
	workCh := make(chan workItem)
	resultCh := make(chan batchResult, len(repos)/max(1, cfg.batchSize)+1)
	var wg sync.WaitGroup

	for range cfg.concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workCh {
				resultCh <- fetchBatch(client, cfg, work.batch)
			}
		}()
	}

	go func() {
		for _, batch := range chunkRefs(repos, cfg.batchSize) {
			workCh <- workItem{batch: batch}
		}
		close(workCh)
		wg.Wait()
		close(resultCh)
	}()

	var counts resultCounts
	for res := range resultCh {
		if err := writer.appendBatch(res); err != nil {
			return counts, err
		}
		counts.wrote += len(res.metas)
		counts.missing += len(res.missing)
		counts.errors += len(res.errors)
	}
	return counts, nil
}

func fetchBatch(client httpClient, cfg config, batch []repoRef) batchResult {
	query := buildGraphQLQuery(batch)
	payload, err := json.Marshal(graphQLRequest{Query: query})
	if err != nil {
		return batchResult{errors: []string{fmt.Sprintf("marshal query: %v", err)}}
	}

	req, err := http.NewRequest(http.MethodPost, cfg.baseURL, bytes.NewReader(payload))
	if err != nil {
		return batchResult{errors: []string{fmt.Sprintf("build request: %v", err)}}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+cfg.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return batchResult{errors: []string{fmt.Sprintf("request failed for batch starting %s: %v", batch[0].Repo, err)}}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return batchResult{errors: []string{fmt.Sprintf("unexpected status %s for batch starting %s", resp.Status, batch[0].Repo)}}
	}

	var gqlResp graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return batchResult{errors: []string{fmt.Sprintf("decode response for batch starting %s: %v", batch[0].Repo, err)}}
	}

	res := batchResult{}
	for _, gqlErr := range gqlResp.Errors {
		if alias, ok := firstStringPath(gqlErr.Path); ok {
			idx := aliasIndex(alias)
			if idx >= 0 && idx < len(batch) {
				if isNotFound(gqlErr) {
					res.missing = append(res.missing, batch[idx].Repo)
					continue
				}
				res.errors = append(res.errors, fmt.Sprintf("%s: %s", batch[idx].Repo, gqlErr.Message))
				continue
			}
		}
		res.errors = append(res.errors, gqlErr.Message)
	}

	seenMissing := make(map[string]struct{}, len(res.missing))
	for _, repo := range res.missing {
		seenMissing[repo] = struct{}{}
	}

	for i, ref := range batch {
		alias := aliasForIndex(i)
		raw, ok := gqlResp.Data[alias]
		if !ok {
			if _, missing := seenMissing[ref.Repo]; !missing {
				res.errors = append(res.errors, fmt.Sprintf("%s: missing GraphQL field %s", ref.Repo, alias))
			}
			continue
		}
		if string(raw) == "null" {
			if _, missing := seenMissing[ref.Repo]; !missing {
				res.missing = append(res.missing, ref.Repo)
			}
			continue
		}

		var repo graphQLRepo
		if err := json.Unmarshal(raw, &repo); err != nil {
			res.errors = append(res.errors, fmt.Sprintf("%s: decode repo payload: %v", ref.Repo, err))
			continue
		}

		meta := repoMetadata{
			FullName:        repo.NameWithOwner,
			Stars:           repo.StargazerCount,
			Forks:           repo.ForkCount,
			Archived:        repo.IsArchived,
			Fork:            repo.IsFork,
			PushedAt:        repo.PushedAt,
			OpenIssuesCount: repo.Issues.TotalCount,
			URL:             repo.URL,
		}
		if repo.DefaultBranchRef != nil {
			meta.DefaultBranch = repo.DefaultBranchRef.Name
		}
		if repo.PrimaryLanguage != nil {
			meta.Language = repo.PrimaryLanguage.Name
		}
		res.metas = append(res.metas, meta)
	}

	return res
}

func buildGraphQLQuery(batch []repoRef) string {
	var b strings.Builder
	b.WriteString("query {")
	for i, ref := range batch {
		fmt.Fprintf(&b, `
%s: repository(owner:%q, name:%q) {
  nameWithOwner
  stargazerCount
  forkCount
  isArchived
  isFork
  pushedAt
  url
  defaultBranchRef { name }
  primaryLanguage { name }
  issues(states: OPEN) { totalCount }
}`, aliasForIndex(i), ref.Owner, ref.Name)
	}
	b.WriteString("\n}")
	return b.String()
}

func openCSVForAppend(path string) (*os.File, bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("open csv: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, false, fmt.Errorf("stat csv: %w", err)
	}
	return f, info.Size() == 0, nil
}

func openTextForAppend(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return f, nil
}

func replaceExt(path, suffix string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path + suffix
	}
	return strings.TrimSuffix(path, ext) + suffix
}

func aliasForIndex(i int) string { return fmt.Sprintf("r%d", i) }

func aliasIndex(alias string) int {
	if len(alias) < 2 || alias[0] != 'r' {
		return -1
	}
	var idx int
	for _, ch := range alias[1:] {
		if ch < '0' || ch > '9' {
			return -1
		}
		idx = idx*10 + int(ch-'0')
	}
	return idx
}

func firstStringPath(path []any) (string, bool) {
	if len(path) == 0 {
		return "", false
	}
	s, ok := path[0].(string)
	return s, ok
}

func isNotFound(err graphQLError) bool {
	if strings.Contains(strings.ToLower(err.Message), "could not resolve to a repository") {
		return true
	}
	if typ, ok := err.Extensions["type"].(string); ok && strings.EqualFold(typ, "NOT_FOUND") {
		return true
	}
	return false
}

func chunkRefs(refs []repoRef, size int) [][]repoRef {
	if size <= 0 {
		size = 1
	}
	out := make([][]repoRef, 0, (len(refs)+size-1)/size)
	for start := 0; start < len(refs); start += size {
		end := start + size
		if end > len(refs) {
			end = len(refs)
		}
		out = append(out, refs[start:end])
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type progress struct {
	done map[string]struct{}
}

func loadProgress(csvPath string) (progress, error) {
	done := make(map[string]struct{})
	for _, path := range []string{csvPath, replaceExt(csvPath, ".missing.txt"), replaceExt(csvPath, ".errors.txt")} {
		if err := loadProgressFile(path, done); err != nil {
			return progress{}, err
		}
	}
	return progress{done: done}, nil
}

func loadProgressFile(path string, done map[string]struct{}) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open progress file %s: %w", path, err)
	}
	defer f.Close()

	if strings.HasSuffix(path, ".csv") {
		r := csv.NewReader(f)
		records, err := r.ReadAll()
		if err != nil {
			return fmt.Errorf("read csv progress %s: %w", path, err)
		}
		for i, record := range records {
			if i == 0 || len(record) == 0 {
				continue
			}
			done["github.com/"+record[0]] = struct{}{}
		}
		return nil
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		repo := progressRepoFromLine(line)
		if repo != "" {
			done[repo] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan progress %s: %w", path, err)
	}
	return nil
}

func progressRepoFromLine(line string) string {
	if strings.HasPrefix(line, "github.com/") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return ""
}

func (p progress) processedCount() int { return len(p.done) }

func filterPending(repos []repoRef, p progress) []repoRef {
	out := repos[:0]
	for _, repo := range repos {
		if _, ok := p.done[repo.Repo]; ok {
			continue
		}
		out = append(out, repo)
	}
	return out
}

type outputWriter struct {
	csvFile   *os.File
	csvWriter *csv.Writer
	missFile  *os.File
	errFile   *os.File
}

func newOutputWriter(csvPath string) (*outputWriter, error) {
	csvFile, writeHeader, err := openCSVForAppend(csvPath)
	if err != nil {
		return nil, err
	}
	csvWriter := csv.NewWriter(csvFile)
	if writeHeader {
		if err := csvWriter.Write([]string{
			"full_name",
			"stars",
			"forks",
			"archived",
			"fork",
			"pushed_at",
			"default_branch",
			"open_issues_count",
			"language",
			"url",
		}); err != nil {
			csvFile.Close()
			return nil, fmt.Errorf("write csv header: %w", err)
		}
		csvWriter.Flush()
		if err := csvWriter.Error(); err != nil {
			csvFile.Close()
			return nil, fmt.Errorf("flush csv header: %w", err)
		}
	}

	missFile, err := openTextForAppend(replaceExt(csvPath, ".missing.txt"))
	if err != nil {
		csvFile.Close()
		return nil, err
	}
	errFile, err := openTextForAppend(replaceExt(csvPath, ".errors.txt"))
	if err != nil {
		csvFile.Close()
		missFile.Close()
		return nil, err
	}

	return &outputWriter{csvFile: csvFile, csvWriter: csvWriter, missFile: missFile, errFile: errFile}, nil
}

func (w *outputWriter) appendBatch(res batchResult) error {
	sort.Slice(res.metas, func(i, j int) bool {
		if res.metas[i].Stars != res.metas[j].Stars {
			return res.metas[i].Stars > res.metas[j].Stars
		}
		if res.metas[i].Fork != res.metas[j].Fork {
			return !res.metas[i].Fork && res.metas[j].Fork
		}
		return res.metas[i].FullName < res.metas[j].FullName
	})
	sort.Strings(res.missing)
	sort.Strings(res.errors)

	for _, row := range res.metas {
		record := []string{
			row.FullName,
			strconv.Itoa(row.Stars),
			strconv.Itoa(row.Forks),
			strconv.FormatBool(row.Archived),
			strconv.FormatBool(row.Fork),
			row.PushedAt,
			row.DefaultBranch,
			strconv.Itoa(row.OpenIssuesCount),
			row.Language,
			row.URL,
		}
		if err := w.csvWriter.Write(record); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}
	w.csvWriter.Flush()
	if err := w.csvWriter.Error(); err != nil {
		return fmt.Errorf("flush csv rows: %w", err)
	}
	if err := w.csvFile.Sync(); err != nil {
		return fmt.Errorf("sync csv: %w", err)
	}

	for _, line := range res.missing {
		if _, err := fmt.Fprintln(w.missFile, line); err != nil {
			return fmt.Errorf("write missing: %w", err)
		}
	}
	if len(res.missing) > 0 {
		if err := w.missFile.Sync(); err != nil {
			return fmt.Errorf("sync missing: %w", err)
		}
	}

	for _, line := range res.errors {
		if _, err := fmt.Fprintln(w.errFile, line); err != nil {
			return fmt.Errorf("write errors: %w", err)
		}
	}
	if len(res.errors) > 0 {
		if err := w.errFile.Sync(); err != nil {
			return fmt.Errorf("sync errors: %w", err)
		}
	}

	return nil
}

func (w *outputWriter) Close() error {
	var errs []string
	if w.csvFile != nil {
		if err := w.csvFile.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if w.missFile != nil {
		if err := w.missFile.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if w.errFile != nil {
		if err := w.errFile.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

type resultCounts struct {
	wrote   int
	missing int
	errors  int
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
