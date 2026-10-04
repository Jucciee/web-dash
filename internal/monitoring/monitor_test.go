package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"homedashboard/internal/domain"
	"homedashboard/internal/webfetch"
)

func TestOnlyExplicitRefreshStartsHTTPChecks(t *testing.T) {
	var hits atomic.Int32
	responseReady := make(chan struct{})
	started := make(chan struct{}, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		started <- struct{}{}
		select {
		case <-responseReady:
			w.WriteHeader(200)
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	client := webfetch.NewClient()
	defer client.CloseIdleConnections()
	monitor := New(client)
	defer monitor.Close()
	links := []domain.Service{{ID: "link", URL: server.URL, WebCheckEnabled: true}}
	if statuses := monitor.Snapshot(links, false); statuses[0].State != "unknown" || statuses[0].Checking {
		t.Fatal("a cached read started a check")
	}
	if hits.Load() != 0 {
		t.Fatal("an idle monitor made a request")
	}
	monitor.Snapshot(links, true)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("page visit did not start an HTTP check")
	}
	for range 10 {
		monitor.Snapshot(links, true)
		monitor.Snapshot(links, false)
	}
	if hits.Load() != 1 {
		t.Fatal("concurrent visits were not coalesced")
	}
	close(responseReady)
	deadline := time.Now().Add(time.Second)
	for monitor.Snapshot(links, false)[0].Checking {
		if time.Now().After(deadline) {
			t.Fatal("HTTP check never completed")
		}
		time.Sleep(time.Millisecond)
	}
	monitor.mu.Lock()
	item := monitor.entries["link"]
	item.started = time.Now().Add(-time.Minute)
	monitor.entries["link"] = item
	monitor.mu.Unlock()
	for range 10 {
		if statuses := monitor.Snapshot(links, false); statuses[0].Checking || statuses[0].State != "online" {
			t.Fatal("reading an old result started another check")
		}
	}
	monitor.Snapshot(links, true)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("the next visit did not refresh the result")
	}
	links[0].WebCheckEnabled = false
	if statuses := monitor.Snapshot(links, true); statuses[0].State != "disabled" || statuses[0].Checking {
		t.Fatal("disabled links were checked")
	}
}

func TestHTTPObservations(t *testing.T) {
	var fallback atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/ok", 302)
		case "/401":
			w.WriteHeader(401)
		case "/403":
			w.WriteHeader(403)
		case "/404":
			w.WriteHeader(404)
		case "/500":
			w.WriteHeader(500)
		case "/head-unsupported":
			if r.Method == "HEAD" {
				w.WriteHeader(405)
			} else {
				fallback.Add(1)
				w.WriteHeader(200)
			}
		case "/timeout":
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
		default:
			w.WriteHeader(200)
		}
	}))
	defer server.Close()
	client := webfetch.NewClient()
	defer client.CloseIdleConnections()
	for _, item := range []struct {
		path, state string
		code        int
	}{{"/ok", "online", 200}, {"/redirect", "online", 200}, {"/401", "auth", 401}, {"/403", "auth", 403}, {"/404", "error", 404}, {"/500", "error", 500}, {"/head-unsupported", "online", 200}} {
		t.Run(item.path, func(t *testing.T) {
			status := Check(t.Context(), client, "link", server.URL+item.path)
			if status.State != item.state || status.HTTPStatus != item.code || status.CheckedAt == nil || status.Checking {
				t.Fatalf("unexpected observation: %+v", status)
			}
		})
	}
	if fallback.Load() != 1 {
		t.Fatal("HEAD fallback did not issue exactly one GET")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if status := Check(ctx, client, "link", server.URL+"/timeout"); status.State != "timeout" {
		t.Fatalf("timeout: %+v", status)
	}
	server.Close()
	if status := Check(t.Context(), client, "link", server.URL); status.State != "offline" {
		t.Fatalf("closed listener: %+v", status)
	}
	if status := Check(t.Context(), client, "link", "http://169.254.169.254/latest/meta-data/"); status.State != "error" {
		t.Fatalf("metadata address: %+v", status)
	}
}

func TestUntrustedTLSIsReported(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	defer server.Close()
	client := webfetch.NewClient()
	defer client.CloseIdleConnections()
	status := Check(t.Context(), client, "link", server.URL)
	if status.State != "error" || status.Message != "TLS 인증서 확인 실패" {
		t.Fatalf("unexpected TLS observation: %+v", status)
	}
}
