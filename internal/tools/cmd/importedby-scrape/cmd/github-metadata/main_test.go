// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRepoRefs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.txt")
	content := strings.Join([]string{
		"github.com/acme/service",
		"github.com/acme/other/pkg",
		"gitlab.com/acme/service",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := readRepoRefs(path)
	if err != nil {
		t.Fatalf("readRepoRefs: %v", err)
	}
	if len(got) != 1 || got[0].Repo != "github.com/acme/service" {
		t.Fatalf("readRepoRefs() = %v, want [github.com/acme/service]", got)
	}
}

func TestBuildGraphQLQuery(t *testing.T) {
	query := buildGraphQLQuery([]repoRef{
		{Repo: "github.com/acme/service", Owner: "acme", Name: "service"},
		{Repo: "github.com/acme/other", Owner: "acme", Name: "other"},
	})
	if !strings.Contains(query, `r0: repository(owner:"acme", name:"service")`) {
		t.Fatalf("query missing first repo: %s", query)
	}
	if !strings.Contains(query, `r1: repository(owner:"acme", name:"other")`) {
		t.Fatalf("query missing second repo: %s", query)
	}
}

func TestFetchBatch(t *testing.T) {
	client := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.github.test/graphql" {
			t.Fatalf("url = %q", req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("Authorization = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body: io.NopCloser(strings.NewReader(`{
				"data": {
					"r0": {
						"nameWithOwner": "acme/service",
						"stargazerCount": 42,
						"forkCount": 7,
						"isArchived": false,
						"isFork": false,
						"pushedAt": "2026-03-10T00:00:00Z",
						"url": "https://github.com/acme/service",
						"defaultBranchRef": {"name": "main"},
						"primaryLanguage": {"name": "Go"},
						"issues": {"totalCount": 3}
					},
					"r1": null
				},
				"errors": [{
					"message": "Could not resolve to a Repository with the name 'acme/missing'.",
					"path": ["r1"],
					"extensions": {"type": "NOT_FOUND"}
				}]
			}`)),
			Header: make(http.Header),
		}, nil
	})

	res := fetchBatch(client, config{
		token:   "token",
		baseURL: "https://api.github.test/graphql",
	}, []repoRef{
		{Repo: "github.com/acme/service", Owner: "acme", Name: "service"},
		{Repo: "github.com/acme/missing", Owner: "acme", Name: "missing"},
	})

	if len(res.metas) != 1 || res.metas[0].FullName != "acme/service" {
		t.Fatalf("unexpected metas: %+v", res.metas)
	}
	if len(res.missing) != 1 || res.missing[0] != "github.com/acme/missing" {
		t.Fatalf("unexpected missing: %+v", res.missing)
	}
	if len(res.errors) != 0 {
		t.Fatalf("unexpected errors: %+v", res.errors)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}
