// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/go-cmp/cmp"
)

func TestExtractImporters(t *testing.T) {
	html := `
<!doctype html>
<html>
  <body>
    <main>
      <a href="/go.opentelemetry.io/otel/attribute?tab=importedby">go.opentelemetry.io/otel/attribute</a>
      <a href="/github.com/example/service/internal/tracing">github.com/example/service/internal/tracing</a>
      <a href="/github.com/example/service/internal/tracing">github.com/example/service/internal/tracing</a>
      <a href="/github.com/acme/app/observability">github.com/acme/app/observability</a>
      <a href="/search?q=go.opentelemetry.io/otel/attribute">Search</a>
      <a href="https://pkg.go.dev/std">std</a>
      <a href="https://example.com/github.com/bad/outside">outside</a>
    </main>
  </body>
</html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("NewDocumentFromReader: %v", err)
	}

	got := extractImporters(doc, "go.opentelemetry.io/otel/attribute")
	want := []string{
		"github.com/acme/app/observability",
		"github.com/example/service/internal/tracing",
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("extractImporters mismatch (-want +got):\n%s", diff)
	}
}

func TestImporterPathFromHref(t *testing.T) {
	tests := []struct {
		name string
		href string
		want string
		ok   bool
	}{
		{name: "package path", href: "/github.com/example/repo/pkg", want: "github.com/example/repo/pkg", ok: true},
		{name: "pkg.go.dev absolute url", href: "https://pkg.go.dev/github.com/example/repo/pkg", want: "github.com/example/repo/pkg", ok: true},
		{name: "query", href: "/github.com/example/repo/pkg?tab=importedby", ok: false},
		{name: "search", href: "/search?q=otel", ok: false},
		{name: "external", href: "https://example.com/github.com/example/repo/pkg", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := importerPathFromHref(tt.href)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("got = %q, want %q", got, tt.want)
			}
		})
	}
}
