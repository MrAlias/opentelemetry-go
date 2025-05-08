// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		name             string
		schema1, schema2 string
		want             string
	}{
		{
			name:    "different schema urls",
			schema1: "https://opentelemetry.io/schemas/1.3.0",
			schema2: "https://opentelemetry.io/schemas/1.4.0",
			want:    "https://opentelemetry.io/schemas/1.4.0",
		},
		{
			name:    "same schema url",
			schema1: "https://opentelemetry.io/schemas/1.4.0",
			schema2: "https://opentelemetry.io/schemas/1.4.0",
			want:    "https://opentelemetry.io/schemas/1.4.0",
		},
		{
			name:    "missing first schema url",
			schema1: "",
			schema2: "https://opentelemetry.io/schemas/1.4.0",
			want:    "https://opentelemetry.io/schemas/1.4.0",
		},
		{
			name:    "missing second schema url",
			schema1: "https://opentelemetry.io/schemas/1.4.0",
			schema2: "",
			want:    "https://opentelemetry.io/schemas/1.4.0",
		},
		{
			name:    "missing both schema url",
			schema1: "",
			schema2: "",
			want:    "",
		},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("case-%s", c.name), func(t *testing.T) {
			d1 := resource.StringDetector(c.schema1, semconv.HostNameKey, os.Hostname)
			d2 := resource.StringDetector(c.schema2, semconv.HostNameKey, os.Hostname)
			r, err := resource.Detect(context.Background(), d1, d2)
			require.NoError(t, err)
			require.NotNil(t, r)
			assert.Equal(t, c.want, r.SchemaURL())
		})
	}
}

func TestMerge2(t *testing.T) {
	cases := []struct {
		name       string
		res1, res2 *resource.Resource
		want       *resource.Resource
	}{
		{
			name: "No upgrade",
			res1: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.18.0",
				attribute.String("res1", "1.18.0"),
				attribute.String("browser.user_agent", "go_test"), //This should not be upgraded to 1.19.0's "user_agent.original"
			),
			res2: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.15.0",
				attribute.String("res2", "1.15.0"),
			),
			want: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.18.0",
				attribute.String("res1", "1.18.0"),
				attribute.String("browser.user_agent", "go_test"),
				attribute.String("res2", "1.15.0"),
			),
		},
		{
			name: "With upgrade",
			res1: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.18.0",
				attribute.String("res1", "1.18.0"),
				attribute.String("browser.user_agent", "go_test"),
			),
			res2: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.19.0",
				attribute.String("res2", "1.19.0"),
			),
			want: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.19.0",
				attribute.String("res1", "1.18.0"),
				attribute.String("user_agent.original", "go_test"),
				attribute.String("res2", "1.19.0"),
			),
		},
		{
			name: "Transient upgrade",
			res1: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.18.0",
				attribute.String("res1", "1.18.0"),
				attribute.String("browser.user_agent", "go_test"),
			),
			res2: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.20.0",
				attribute.String("res2", "1.20.0"),
			),
			want: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.20.0",
				attribute.String("res1", "1.18.0"),
				attribute.String("user_agent.original", "go_test"),
				attribute.String("res2", "1.20.0"),
			),
		},
		{
			name: "Doesn't upgrade outside of resources",
			res1: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.18.0",
				attribute.String("res1", "1.18.0"),
				attribute.String("faas.execution", "af47"), // This is a span attribute it shouldn't be modified by Merge
			),
			res2: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.20.0",
				attribute.String("res2", "1.20.0"),
			),
			want: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.20.0",
				attribute.String("res1", "1.18.0"),
				attribute.String("faas.execution", "af47"),
				attribute.String("res2", "1.20.0"),
			),
		},
		{
			name: "Doesn't upgrade previous version transitions",
			res1: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.20.0",
				attribute.String("res1", "1.20.0"),
				attribute.String("browser.user_agent", "go_test"),
			),
			res2: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.21.0",
				attribute.String("res2", "1.21.0"),
			),
			want: resource.NewWithAttributes("https://opentelemetry.io/schemas/1.21.0",
				attribute.String("res1", "1.20.0"),
				attribute.String("browser.user_agent", "go_test"),
				attribute.String("res2", "1.21.0"),
			),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resource.Merge(tt.res1, tt.res2)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			got, err = resource.Merge(tt.res2, tt.res1)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
