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

package main

import (
	"flag"
	"log"

	"go.opentelemetry.io/otel/sdk/resource/internal/schema/generate/cmd"
)

// local is the CLI flag for the local schema directory.
var local string

func init() {
	flag.StringVar(&local, "local", "", "local schema directory to use instead of fetching remote")

	flag.Parse()
}

func main() {
	dest := flag.Arg(0)
	if dest == "" {
		log.Fatalln("empty desination")
	}

	if err := cmd.Run(dest, local); err != nil {
		log.Fatal(err)
	}
}