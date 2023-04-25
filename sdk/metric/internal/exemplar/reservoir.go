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

package exemplar // import "go.opentelemetry.io/otel/sdk/metric/internal/exemplar"

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// Reservoir holds the sampled exemplar of measurements made.
type Reservoir[N int64 | float64] interface {
	// Offer accepts the parameters associated with a measurment. The
	// parameters will be stored as an exemplar if the Reservoir decides to
	// sample the measurment.
	Offer(context.Context, time.Time, N, attribute.Set)

	// Collect returns all the held exemplars with each exemplars dropped
	// attributes updated to include any attributes the Filter filters out.
	//
	// The Reservoir state is preserved after this call. See Flush to
	// copy-and-clear instead.
	Collect() map[attribute.Set][]Measurement[N]

	// Flush returns all the held exemplars with each exemplars dropped
	// attributes updated to include any attributes the Filter filters out.
	//
	// The Reservoir state is reset after this call. See Collect to preserve
	// the state instead.
	Flush() map[attribute.Set][]Measurement[N]
}
