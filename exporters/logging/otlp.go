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
	"io"

	"go.opentelemetry.io/otel/internal/otlp/transform"
	"go.opentelemetry.io/otel/sdk/trace"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	jsonpb "google.golang.org/protobuf/encoding/protojson"
)

type OTLPMarshaler struct {
	JSONMarshalOptions jsonpb.MarshalOptions
}

func (m OTLPMarshaler) SpanMarshal(w io.Writer, spans []*trace.SpanSnapshot) error {
	protoSpans := transform.SpanData(spans)
	if len(protoSpans) == 0 {
		return nil
	}
	pbRequest := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: protoSpans,
	}
	b, err := m.JSONMarshalOptions.Marshal(pbRequest)
	if err != nil {
		return nil
	}
	_, err = w.Write(b)
	return err
}
