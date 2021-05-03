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

package logging

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/otel/sdk/trace"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/protobuf/encoding/protojson"
)

type Printer struct{}

func (p Printer) SpanMarshal(w io.Writer, spans []*tracesdk.SpanSnapshot) error {
	for _, s := range spans {
		fmt.Fprintf(w, "%+v\n", s)
	}
	return nil
}

func Example() {
	m := OTLPMarshaler{JSONMarshalOptions: protojson.MarshalOptions{Indent: "\t"}}
	exp := NewExporter(os.Stdout, m)
	tp := trace.NewTracerProvider(trace.WithSyncer(exp))
	tracer := tp.Tracer("test")

	ctx := context.Background()
	_, s := tracer.Start(ctx, "first")
	s.End()

	// Output:
	//
}
