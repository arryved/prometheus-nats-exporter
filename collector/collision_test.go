// Copyright 2026 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// collidingVarz mirrors real keys seen on an Arryved portal host: distinct
// JSON keys that sanitize to the same Prometheus metric name.
const collidingVarz = `{
  "server_id": "test",
  "unique.name": 8,
  "call": {
    "docker-compose.dev.yml.success": {"avg": 1, "count": 2},
    "docker-compose-dev.yml.success": {"avg": 3, "count": 4},
    "\\.success": 6,
    ").success": 7,
    "log.error": 10,
    "_log.error": 9
  }
}`

func TestVarzSanitizedNameCollisions(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/varz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(collidingVarz))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	servers := []*CollectedServer{{ID: "id", URL: ts.URL}}
	reg := prometheus.NewRegistry()
	if err := reg.Register(NewCollector(CoreSystem, "varz", "arryved", servers)); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Gather twice: the second pass goes through makeRequests with the same keys.
	for pass := 1; pass <= 2; pass++ {
		mfs, err := reg.Gather()
		if err != nil {
			t.Fatalf("pass %d: gather failed: %v", pass, err)
		}
		got := map[string]float64{}
		for _, mf := range mfs {
			if n := len(mf.GetMetric()); n != 1 {
				t.Fatalf("pass %d: %s has %d series, want 1", pass, mf.GetName(), n)
			}
			got[mf.GetName()] = mf.GetMetric()[0].GetGauge().GetValue()
		}
		// The first key in lexicographic order wins: '-' < '.', ')' < '\\', '_' < 'l'.
		want := map[string]float64{
			"arryved_varz_unique_name":                               8,
			"arryved_varz_call_docker_compose_dev_yml_success_avg":   3,
			"arryved_varz_call_docker_compose_dev_yml_success_count": 4,
			"arryved_varz_call___success":                            7,
			"arryved_varz_call_log_error":                            9,
		}
		for name, v := range want {
			if gv, ok := got[name]; !ok || gv != v {
				t.Errorf("pass %d: %s = %v (present=%v), want %v", pass, name, gv, ok, v)
			}
		}
		if len(got) != len(want)+1 { // +1 for server_id label gauge
			t.Errorf("pass %d: got %d metric families, want %d: %v", pass, len(got), len(want)+1, got)
		}
	}
}
