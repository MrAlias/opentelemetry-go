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
	"context"
	"sync"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type Gatherer interface {
	Gather(context.Context) error
	Storage() *Storage
}

// Storage stores exported metric data.
type Storage struct {
	dataMu sync.Mutex
	data   []metricdata.ResourceMetrics
}

// NewStorage returns a configures Storage ready to store exported metric data.
func NewStorage() *Storage {
	return &Storage{}
}

// Add adds the request to the Storage.
func (s *Storage) Add(data metricdata.ResourceMetrics) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	s.data = append(s.data, data)
}

// dump returns all added metric data and clears the storage.
func (s *Storage) dump() []metricdata.ResourceMetrics {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	var data []metricdata.ResourceMetrics
	data, s.data = s.data, []metricdata.ResourceMetrics{}
	return data
}
