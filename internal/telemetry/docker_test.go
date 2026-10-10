package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
)

func newDockerTestCollector(t *testing.T, rows []container.Summary, stats http.HandlerFunc) *Collector {
	t.Helper()
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_ping" {
			w.Header().Set("API-Version", "1.56")
			_, _ = w.Write([]byte("OK"))
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/info"):
			_, _ = w.Write([]byte(`{"Name":"fixture-engine","NCPU":4}`))
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			_ = json.NewEncoder(w).Encode(rows)
		case strings.HasSuffix(r.URL.Path, "/stats"):
			if r.URL.Query().Get("stream") != "false" || r.URL.Query().Get("one-shot") != "true" {
				t.Error("stats must request immediate one-shot samples")
			}
			stats(w, r)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	t.Cleanup(engine.Close)
	collector := New(Config{DockerHost: "tcp://" + strings.TrimPrefix(engine.URL, "http://")})
	t.Cleanup(collector.Close)
	return collector
}

func TestDockerCollectsNineteenContainersInTwoSweeps(t *testing.T) {
	const count = 19
	rows := make([]container.Summary, count)
	indices := make(map[string]int, count)
	for index := range rows {
		id := fmt.Sprintf("%064x", index+1)
		rows[index] = container.Summary{ID: id, Names: []string{fmt.Sprintf("/app-%02d", index)}, State: "running"}
		indices[id] = index
	}
	var mu sync.Mutex
	var requests [count]int
	var first, second [count]time.Time
	active, maxActive := 0, 0
	collector := newDockerTestCollector(t, rows, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/stats")
		index := indices[path[strings.LastIndex(path, "/")+1:]]
		mu.Lock()
		requests[index]++
		request := requests[index]
		if request == 1 {
			first[index] = time.Now()
		} else if request == 2 {
			second[index] = time.Now()
			for _, started := range first {
				if started.IsZero() {
					t.Error("second sweep started before every container had a baseline")
				}
			}
		} else {
			t.Errorf("container %d sampled %d times", index, request)
		}
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		defer func() {
			mu.Lock()
			active--
			mu.Unlock()
		}()
		// Simulate a small Engine round-trip for each of the five worker batches.
		time.Sleep(20 * time.Millisecond)
		_, _ = fmt.Fprintf(w, `{"read":"2026-10-09T00:00:0%dZ","cpu_stats":{"cpu_usage":{"total_usage":%d},"system_cpu_usage":%d,"online_cpus":4},"memory_stats":{"usage":300,"limit":1000,"stats":{"inactive_file":100}},"networks":{"eth0":{"rx_bytes":150,"tx_bytes":225}}}`, request-1, 1000+250*(request-1), 1000*request)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	started := time.Now()
	snapshot := collector.collectDocker(ctx)
	elapsed := time.Since(started)
	t.Logf("19 containers collected in %s", elapsed)
	if ctx.Err() != nil {
		t.Fatalf("collection exceeded its three-second budget: %v", ctx.Err())
	}
	if !snapshot.Available || snapshot.Running != count || len(snapshot.Containers) != count {
		t.Fatalf("incomplete snapshot: %+v", snapshot)
	}
	for _, item := range snapshot.Containers {
		if item.StatsError != "" || item.CPUPercent == nil || *item.CPUPercent != 100 || item.Memory == nil || item.Memory.Used != 200 || item.NetworkRX == nil || *item.NetworkRX != 150 || item.NetworkTX == nil || *item.NetworkTX != 225 {
			t.Errorf("incomplete resources: %+v", item)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for index := range requests {
		if requests[index] != 2 || second[index].Sub(first[index]) < time.Second {
			t.Errorf("container %d: %d samples, interval %s", index, requests[index], second[index].Sub(first[index]))
		}
	}
	if maxActive < 2 || maxActive > 4 {
		t.Errorf("stats concurrency %d, want 2 to 4", maxActive)
	}
}

func TestDockerKeepsResourcesWhenOneSampleFails(t *testing.T) {
	for _, failedSample := range []int32{1, 2} {
		t.Run(fmt.Sprintf("sample-%d", failedSample), func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			collector := newDockerTestCollector(t, []container.Summary{{ID: strings.Repeat("a", 64), Names: []string{"/app"}, State: "running"}}, func(w http.ResponseWriter, r *http.Request) {
				request := requests.Add(1)
				if request == failedSample {
					if failedSample == 1 {
						_, _ = w.Write([]byte(`{"read":`))
					} else {
						w.WriteHeader(http.StatusForbidden)
					}
					return
				}
				_, _ = fmt.Fprintf(w, `{"read":"2026-10-09T00:00:0%dZ","cpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":2000,"online_cpus":4},"memory_stats":{"usage":300,"limit":1000,"stats":{"inactive_file":100}},"networks":{"eth0":{"rx_bytes":150,"tx_bytes":225}}}`, request-1)
			})
			snapshot := collector.collectDocker(t.Context())
			if !snapshot.Available || len(snapshot.Containers) != 1 || requests.Load() != 2 {
				t.Fatalf("snapshot: %+v, requests: %d", snapshot, requests.Load())
			}
			item := snapshot.Containers[0]
			if item.StatsError == "" || item.CPUPercent != nil || item.Memory == nil || item.Memory.Used != 200 || item.NetworkRX == nil || *item.NetworkRX != 150 || item.NetworkTX == nil || *item.NetworkTX != 225 {
				t.Fatalf("single successful sample must keep resources without CPU: %+v", item)
			}
		})
	}
}

func TestDockerCancellationInterruptsSampleInterval(t *testing.T) {
	var requests atomic.Int32
	collector := newDockerTestCollector(t, []container.Summary{{ID: strings.Repeat("a", 64), Names: []string{"/app"}, State: "running"}}, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"read":"2026-10-09T00:00:00Z","memory_stats":{"usage":100,"limit":1000}}`))
	})
	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	snapshot := collector.collectDocker(ctx)
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Errorf("cancellation did not interrupt the sampling interval: %s", elapsed)
	}
	if len(snapshot.Containers) != 1 || requests.Load() != 1 {
		t.Fatalf("canceled collection started another sweep: %+v, requests: %d", snapshot, requests.Load())
	}
	item := snapshot.Containers[0]
	if item.StatsError == "" || item.CPUPercent != nil || item.Memory == nil || item.Memory.Used != 100 {
		t.Fatalf("canceled interval must preserve baseline resources: %+v", item)
	}
}

func TestDockerStoppedContainersSkipSampleInterval(t *testing.T) {
	collector := newDockerTestCollector(t, []container.Summary{{ID: strings.Repeat("a", 64), Names: []string{"/app"}, State: "exited"}}, func(w http.ResponseWriter, r *http.Request) {
		t.Error("stopped container must not request stats")
	})
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	snapshot := collector.collectDocker(ctx)
	if ctx.Err() != nil || !snapshot.Available || snapshot.Running != 0 || snapshot.Stopped != 1 || snapshot.Containers[0].StatsError != "" {
		t.Fatalf("stopped containers must return without waiting: %+v, %v", snapshot, ctx.Err())
	}
}
