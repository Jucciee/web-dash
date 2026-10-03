package monitoring

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"homedashboard/internal/domain"
	"homedashboard/internal/storage"
	"homedashboard/internal/webfetch"
)

const Interval = 30 * time.Second

type entry struct {
	url     string
	enabled bool
	status  domain.WebStatus
	started time.Time
}
type job struct{ id, url string }
type Monitor struct {
	store   *storage.Repository
	client  *http.Client
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	entries map[string]entry
	jobs    chan job
}

func New(store *storage.Repository, client *http.Client) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Monitor{store: store, client: client, ctx: ctx, cancel: cancel, entries: map[string]entry{}, jobs: make(chan job, 512)}
	for range 4 {
		m.wg.Go(m.worker)
	}
	m.wg.Go(func() {
		m.scan()
		ticker := time.NewTicker(Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.scan()
			}
		}
	})
	return m
}

func (m *Monitor) Close() { m.cancel(); m.wg.Wait() }

func (m *Monitor) scan() {
	links, err := m.store.MonitorLinks(m.ctx)
	if err != nil {
		if m.ctx.Err() == nil {
			slog.Warn("could not load links for web checks")
		}
		return
	}
	m.Snapshot(links, false)
	live := make(map[string]bool, len(links))
	for _, link := range links {
		live[link.ID] = true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.entries {
		if !live[id] {
			delete(m.entries, id)
		}
	}
}

// Snapshot queues missing or stale observations without holding up the page.
// Manual refreshes are coalesced and limited to one start per link per 5s.
func (m *Monitor) Snapshot(links []domain.Service, force bool) []domain.WebStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]domain.WebStatus, 0, len(links))
	now := time.Now().UTC()
	for _, link := range links {
		item, found := m.entries[link.ID]
		if !found || item.url != link.URL || item.enabled != link.WebCheckEnabled {
			item = entry{url: link.URL, enabled: link.WebCheckEnabled, status: domain.WebStatus{ServiceID: link.ID, State: "unknown", Message: "확인 대기"}}
		}
		if !item.enabled {
			item.status = domain.WebStatus{ServiceID: link.ID, State: "disabled", Message: "상태 확인 꺼짐"}
		} else if !item.status.Checking && (now.Sub(item.started) >= Interval || (force && now.Sub(item.started) >= 5*time.Second)) {
			select {
			case m.jobs <- job{link.ID, link.URL}:
				item.started, item.status.Checking = now, true
				if item.status.CheckedAt == nil {
					item.status.State, item.status.Message = "checking", "확인 중"
				}
			default:
			}
		}
		m.entries[link.ID] = item
		result = append(result, item.status)
	}
	return result
}

func (m *Monitor) worker() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case task := <-m.jobs:
			m.mu.Lock()
			current, exists := m.entries[task.id]
			m.mu.Unlock()
			if !exists || current.url != task.url || !current.enabled {
				continue
			}
			status := Check(m.ctx, m.client, task.id, task.url)
			if m.ctx.Err() != nil {
				return
			}
			m.mu.Lock()
			current, exists = m.entries[task.id]
			if exists && current.url == task.url && current.enabled {
				before := current.status.State
				current.status = status
				m.entries[task.id] = current
				if before != status.State {
					slog.Info("web status changed", "linkId", task.id, "state", status.State, "httpStatus", status.HTTPStatus, "durationMs", status.LatencyMS)
				}
			}
			m.mu.Unlock()
		}
	}
}

func Check(ctx context.Context, client *http.Client, id, address string) domain.WebStatus {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	start := time.Now()
	status := domain.WebStatus{ServiceID: id}
	response, err := webfetch.Request(ctx, client, http.MethodHead, address)
	if err == nil && (response.StatusCode == http.StatusMethodNotAllowed || response.StatusCode == http.StatusNotImplemented) {
		response.Body.Close()
		response, err = webfetch.Request(ctx, client, http.MethodGet, address)
	}
	now := time.Now().UTC()
	status.CheckedAt, status.LatencyMS = &now, time.Since(start).Milliseconds()
	if err != nil {
		if response != nil {
			response.Body.Close()
		}
		status.State, status.Message = "offline", "연결 실패"
		var timeout net.Error
		var certificate *tls.CertificateVerificationError
		switch {
		case errors.Is(err, webfetch.ErrBlocked):
			status.State, status.Message = "error", "확인할 수 없는 네트워크 주소"
		case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()):
			status.State, status.Message = "timeout", "응답 시간 초과"
		case errors.As(err, &certificate):
			status.State, status.Message = "error", "TLS 인증서 확인 실패"
		}
		return status
	}
	defer response.Body.Close()
	status.HTTPStatus = response.StatusCode
	switch {
	case response.StatusCode >= 200 && response.StatusCode < 400:
		status.State, status.Message = "online", "정상 응답"
	case response.StatusCode == 401 || response.StatusCode == 403:
		status.State, status.Message = "auth", "인증 필요"
	default:
		status.State, status.Message = "error", "HTTP 오류"
	}
	return status
}
