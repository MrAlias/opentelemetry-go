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

package metrictest // import "go.opentelemetry.io/otel/sdk/metric/metrictest"

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

const (
	pkgName   = "go.opentelemetry.io/otel/sdk/metric/metrictest"
	version   = "v0.1.0"
	schemaURL = "https://opentelemetry.io/schemas/1.0.0"
)

var (
	alice = []attribute.KeyValue{
		attribute.String("username", "alice"),
		attribute.Int("id", 3214986),
		attribute.Bool("active", true),
	}
	bob = []attribute.KeyValue{
		attribute.String("username", "bob"),
		attribute.Int("id", -1),
		attribute.Bool("active", false),
	}

	defaultRes = resource.NewSchemaless(
		attribute.String("company", "globotron"),
	)
	defualtConf = Config{
		Resource: defaultRes,
	}

	defualtScope = instrumentation.Scope{
		Name:      pkgName,
		Version:   version,
		SchemaURL: schemaURL,
	}
)

type Config struct {
	Resource    *resource.Resource
	Temporality metricdata.Temporality
	Aggregation aggregation.Aggregation
}
