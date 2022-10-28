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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric"
)

func testMeter(f Factory) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("CreationConcurrentSafe", func(t *testing.T) {
			mp, _ := f(defualtConf)

			var wg sync.WaitGroup
			wg.Add(1)
			var asyncMeter metric.Meter
			go func() {
				defer wg.Done()
				asyncMeter = mp.Meter(pkgName)
			}()
			syncMeter := mp.Meter(pkgName)
			wg.Wait()

			assert.Same(t, asyncMeter, syncMeter)
		})
	}
}
