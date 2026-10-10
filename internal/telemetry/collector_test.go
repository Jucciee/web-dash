package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"homedashboard/internal/domain"
)

func TestDockerCollectsAllStatesResourcesAndSharesConcurrentReads(t *testing.T) {
	var lists atomic.Int32
	var statsReads atomic.Int32
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_ping" {
			w.Header().Set("API-Version", "1.56")
			_, _ = w.Write([]byte("OK"))
			return
		}
		if r.Method != "GET" {
			t.Errorf("unexpected write: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/info"):
			_, _ = w.Write([]byte(`{"Name":"fixture-engine","OperatingSystem":"Docker Desktop","OSType":"linux","NCPU":4,"MemTotal":8589934592,"ServerVersion":"29.0.0"}`))
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			lists.Add(1)
			if r.URL.Query().Get("all") != "1" {
				t.Error("stopped containers were not requested")
			}
			_, _ = w.Write([]byte(`[{"Id":"` + strings.Repeat("a", 64) + `","Names":["/app"],"State":"running","Status":"Up 1 hour (healthy)","Image":"example/app","Labels":{"com.docker.compose.project":"home"}},{"Id":"` + strings.Repeat("b", 64) + `","Names":["/worker"],"State":"exited","Status":"Exited (0)"},{"Id":"` + strings.Repeat("c", 64) + `","Names":["/partial"],"State":"running","Health":{"Status":"unhealthy"}}]`))
		case strings.Contains(r.URL.Path, strings.Repeat("c", 64)):
			w.WriteHeader(403)
		case strings.HasSuffix(r.URL.Path, "/stats"):
			if r.URL.Query().Get("stream") != "false" || r.URL.Query().Get("one-shot") != "true" {
				t.Error("stats must request immediate one-shot samples")
			}
			if statsReads.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"read":"2026-10-09T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":1000,"online_cpus":4},"memory_stats":{"usage":100,"limit":1000},"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20}}}`))
			} else {
				_, _ = w.Write([]byte(`{"read":"2026-10-09T00:00:01Z","cpu_stats":{"cpu_usage":{"total_usage":1250},"system_cpu_usage":2000,"online_cpus":4},"memory_stats":{"usage":300,"limit":1000,"stats":{"inactive_file":100}},"networks":{"eth0":{"rx_bytes":100,"tx_bytes":200},"eth1":{"rx_bytes":50,"tx_bytes":25}}}`))
			}
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(403)
		}
	}))
	defer engine.Close()
	collector := New(Config{DockerHost: "tcp://" + strings.TrimPrefix(engine.URL, "http://")})
	defer collector.Close()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			snapshot, err := collector.Docker(t.Context())
			if err != nil {
				t.Error(err)
				return
			}
			if !snapshot.Available || snapshot.Running != 2 || snapshot.Stopped != 1 || snapshot.Name != "fixture-engine" || snapshot.OS != "Docker Desktop" {
				t.Errorf("snapshot: %+v", snapshot)
				return
			}
			app := snapshot.Containers[0]
			if app.Name != "app" || app.Health != "healthy" || app.CPUPercent == nil || *app.CPUPercent != 100 || app.Memory == nil || app.Memory.Used != 200 || app.Memory.Percent != 20 || app.NetworkRX == nil || *app.NetworkRX != 150 || *app.NetworkTX != 225 {
				t.Errorf("resources: %+v", app)
			}
			if snapshot.Containers[1].StatsError == "" || snapshot.Containers[1].CPUPercent != nil {
				t.Error("partial failure must not fabricate usage")
			}
			if snapshot.Containers[2].State != "exited" || snapshot.Containers[2].Memory != nil {
				t.Error("stopped container must remain visible without usage")
			}
		})
	}
	wg.Wait()
	if lists.Load() != 1 {
		t.Errorf("concurrent reads hit Engine %d times", lists.Load())
	}
	if statsReads.Load() != 2 {
		t.Errorf("concurrent reads took %d samples, want 2", statsReads.Load())
	}
}

func TestDockerDisconnectedIsAnExplicitEmptySnapshot(t *testing.T) {
	collector := New(Config{DockerHost: "invalid://example"})
	defer collector.Close()
	snapshot, err := collector.Docker(t.Context())
	if err != nil || snapshot.Available || snapshot.Message == "" || len(snapshot.Containers) != 0 {
		t.Fatalf("unavailable: %+v %v", snapshot, err)
	}
}

func TestDockerHookRejectsControlSecretsAndUnboundedEndpoints(t *testing.T) {
	for _, input := range []struct{ method, path string }{
		{"POST", "/v1.56/containers/" + strings.Repeat("a", 64) + "/start"}, {"DELETE", "/v1.56/containers/" + strings.Repeat("a", 64)},
		{"GET", "/v1.56/containers/" + strings.Repeat("a", 64) + "/json"}, {"GET", "/v1.56/containers/" + strings.Repeat("a", 64) + "/logs"}, {"GET", "/v1.56/secrets"},
	} {
		r, _ := http.NewRequest(input.method, "http://engine"+input.path, nil)
		if readOnlyDocker(r) == nil {
			t.Fatalf("permitted %s %s", input.method, input.path)
		}
	}
	for _, path := range []string{"/_ping", "/version", "/v1.56/info", "/v1.56/containers/json", "/v1.56/containers/" + strings.Repeat("a", 64) + "/stats"} {
		r, _ := http.NewRequest("GET", "http://engine"+path, nil)
		if err := readOnlyDocker(r); err != nil {
			t.Fatal(path, err)
		}
	}
}

func TestMissingAndResetStatsDoNotBecomeZeroOrOverflow(t *testing.T) {
	for _, raw := range []struct{ sample, baseline string }{
		{`{}`, `{}`},
		{`{"read":"2026-10-09T00:00:01Z","preread":"2026-10-09T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":200,"online_cpus":4},"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":100}}`, `{}`},
		{`{"read":"2026-10-09T00:00:01Z","cpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":200,"online_cpus":4}}`, `{"read":"2026-10-09T00:00:00Z"}`},
		{`{"read":"2026-10-09T00:00:01Z","cpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":300,"online_cpus":4}}`, `{"read":"2026-10-09T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":200}}`},
		{`{"read":"2026-10-09T00:00:01Z","cpu_stats":{"cpu_usage":{"total_usage":300},"system_cpu_usage":100,"online_cpus":4}}`, `{"read":"2026-10-09T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":200}}`},
		{`{"read":"2026-10-09T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":300},"system_cpu_usage":300,"online_cpus":4}}`, `{"read":"2026-10-09T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":200}}`},
	} {
		var sample, baseline container.StatsResponse
		if err := json.Unmarshal([]byte(raw.sample), &sample); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(raw.baseline), &baseline); err != nil {
			t.Fatal(err)
		}
		var item domain.ContainerSnapshot
		applyStats(&item, sample, baseline)
		if item.CPUPercent != nil || item.Memory != nil || item.NetworkRX != nil {
			t.Fatalf("fabricated metrics: %+v", item)
		}
	}
}

func TestDockerIdleCPUIsZeroWithPerCPUCoreFallback(t *testing.T) {
	var sample, baseline container.StatsResponse
	if err := json.Unmarshal([]byte(`{"read":"2026-10-09T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":0},"system_cpu_usage":1000}}`), &baseline); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"read":"2026-10-09T00:00:01Z","cpu_stats":{"cpu_usage":{"total_usage":0,"percpu_usage":[0,0,0,0]},"system_cpu_usage":2000}}`), &sample); err != nil {
		t.Fatal(err)
	}
	var item domain.ContainerSnapshot
	applyStats(&item, sample, baseline)
	if item.CPUPercent == nil || *item.CPUPercent != 0 {
		t.Fatalf("idle container must report a measured zero: %+v", item)
	}
}

func TestCancelledTabDoesNotCancelSharedSample(t *testing.T) {
	var cache cache[int]
	started, finish := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	var calls atomic.Int32
	collect := func(context.Context) int { calls.Add(1); close(started); <-finish; return 42 }
	done := make(chan error, 1)
	go func() { _, err := cache.get(ctx, t.Context(), collect); done <- err }()
	<-started
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatal(err)
	}
	close(finish)
	value, err := cache.get(t.Context(), t.Context(), collect)
	if err != nil || value != 42 || calls.Load() != 1 {
		t.Fatalf("shared sample: %d %v %d", value, err, calls.Load())
	}
}

func TestHostCollectsRealSystemAndMarksDiskFailure(t *testing.T) {
	collector := New(Config{HostName: "test-host", HostDisk: t.TempDir() + "/missing"})
	defer collector.Close()
	snapshot, err := collector.Host(t.Context())
	if err != nil || snapshot.Name != "test-host" || snapshot.MeasuredAt.IsZero() || snapshot.Disk != nil || len(snapshot.Warnings) == 0 {
		t.Fatalf("host: %+v %v", snapshot, err)
	}
	if snapshot.Memory == nil || snapshot.Memory.Total == 0 || snapshot.CPUPercent == nil || *snapshot.CPUPercent < 0 || *snapshot.CPUPercent > 100 {
		t.Fatalf("missing real resources: %+v", snapshot)
	}
	if time.Since(snapshot.MeasuredAt) > time.Minute {
		t.Fatal("stale measurement")
	}
}
