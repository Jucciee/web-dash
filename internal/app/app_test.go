package app_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"homedashboard/internal/app"
	"homedashboard/internal/domain"
	"homedashboard/internal/storage"
)

const setupToken = "integration-test-only-setup-token-32-characters"

type browser struct {
	client *http.Client
	base   string
	csrf   string
}

func newBrowser(base string) *browser {
	jar, _ := cookiejar.New(nil)
	return &browser{client: &http.Client{Jar: jar}, base: base}
}

func (b *browser) request(t *testing.T, method, path string, payload any, revision int64, want int) json.RawMessage {
	t.Helper()
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, b.base+"/api/v1"+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if b.csrf != "" && method != "GET" {
		req.Header.Set("X-CSRF-Token", b.csrf)
	}
	if revision > 0 {
		req.Header.Set("If-Match", fmt.Sprintf("\"%d\"", revision))
	}
	response, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode != want {
		t.Fatalf("%s %s: want %d, got %d: %s", method, path, want, response.StatusCode, data)
	}
	if want == 204 {
		if len(data) > 0 {
			t.Fatal("204 response has a body")
		}
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
		Meta struct {
			RequestID string `json:"requestId"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("invalid response: %s", data)
	}
	if envelope.Meta.RequestID == "" {
		t.Fatal("missing request ID")
	}
	return envelope.Data
}

func decode[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func (b *browser) getCSRF(t *testing.T) {
	result := decode[struct {
		Token string `json:"csrfToken"`
	}](t, b.request(t, "GET", "/auth/csrf", nil, 0, 200))
	b.csrf = result.Token
	if b.csrf == "" {
		t.Fatal("empty CSRF token")
	}
}

func start(t *testing.T, cfg app.Config) (*app.App, *httptest.Server) {
	t.Helper()
	instance, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return instance, httptest.NewServer(instance.Handler)
}

func TestSetupCardsSessionAndPersistence(t *testing.T) {
	cfg := app.Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), SetupToken: setupToken}
	instance, server := start(t, cfg)
	t.Cleanup(func() {
		server.Close()
		if err := instance.Close(); err != nil {
			t.Error(err)
		}
	})
	b := newBrowser(server.URL)
	status := decode[struct {
		NeedsSetup bool `json:"needsSetup"`
	}](t, b.request(t, "GET", "/setup/status", nil, 0, 200))
	if !status.NeedsSetup {
		t.Fatal("new installation must need setup")
	}
	credentials := map[string]string{"setupToken": setupToken, "username": "owner", "password": "correct horse battery staple"}
	b.request(t, "POST", "/setup", credentials, 0, 403)
	b.getCSRF(t)
	b.request(t, "POST", "/services", map[string]any{}, 0, 401)
	wrong := map[string]string{"setupToken": "wrong-token", "username": "owner", "password": "correct horse battery staple"}
	b.request(t, "POST", "/setup", wrong, 0, 401)
	b.request(t, "POST", "/setup", credentials, 0, 201)
	b.request(t, "GET", "/me", nil, 0, 200)
	dashboards := decode[[]domain.Dashboard](t, b.request(t, "GET", "/dashboards", nil, 0, 200))
	if len(dashboards) != 1 {
		t.Fatalf("want one dashboard, got %d", len(dashboards))
	}
	id := dashboards[0].ID
	load := func() domain.Dashboard {
		return decode[domain.Dashboard](t, b.request(t, "GET", "/dashboards/"+id, nil, 0, 200))
	}
	dashboard := load()
	if len(dashboard.Groups) != 1 || len(dashboard.Groups[0].Services) != 0 {
		t.Fatal("missing default empty group")
	}
	group := dashboard.Groups[0]
	input := domain.ServiceInput{GroupID: group.ID, Name: "사진 서버", URL: "https://photos.example.com/Albums/?filter=Recent", IconKey: "photos", Description: "사진 보관"}
	invalid := input
	invalid.URL = "javascript:alert(1)"
	b.request(t, "POST", "/services", invalid, 0, 400)
	invalid.URL = "https://user:password@example.com"
	b.request(t, "POST", "/services", invalid, 0, 400)
	card := decode[domain.Service](t, b.request(t, "POST", "/services", input, 0, 201))
	if card.URL != input.URL || card.Revision != 1 {
		t.Fatal("URL meaning or initial revision changed")
	}
	card = decode[domain.Service](t, b.request(t, "PATCH", "/services/"+card.ID, map[string]any{"hidden": true, "description": ""}, card.Revision, 200))
	if !card.Hidden || card.Description != "" || card.Revision != 2 {
		t.Fatal("zero values or revision not saved")
	}
	b.request(t, "PATCH", "/services/"+card.ID, map[string]any{"name": "stale"}, 1, 409)
	b.request(t, "DELETE", "/services/"+card.ID, nil, 1, 409)
	card = decode[domain.Service](t, b.request(t, "PATCH", "/services/"+card.ID, map[string]any{"hidden": false}, card.Revision, 200))
	if card.Hidden {
		t.Fatal("false value not saved")
	}
	input.Name = "파일 서버"
	input.URL = "http://192.168.1.2:8080/"
	input.IconKey = "files"
	second := decode[domain.Service](t, b.request(t, "POST", "/services", input, 0, 201))
	other := decode[domain.Group](t, b.request(t, "POST", "/dashboards/"+id+"/groups", map[string]string{"name": "미디어"}, 0, 201))
	dashboard = load()
	b.request(t, "PUT", "/dashboards/"+id+"/layout", domain.Layout{Groups: []domain.LayoutGroup{{GroupID: other.ID, ServiceIDs: []string{second.ID}}}}, dashboard.Revision, 400)
	if got := load(); got.Revision != dashboard.Revision || got.Groups[0].ID != group.ID {
		t.Fatal("invalid layout was not rolled back")
	}
	layout := domain.Layout{Groups: []domain.LayoutGroup{{GroupID: group.ID, ServiceIDs: []string{second.ID, card.ID}}, {GroupID: other.ID, ServiceIDs: []string{}}}}
	dashboard = decode[domain.Dashboard](t, b.request(t, "PUT", "/dashboards/"+id+"/layout", layout, dashboard.Revision, 200))
	if dashboard.Groups[0].Services[0].ID != second.ID || dashboard.Groups[0].Services[1].Revision != card.Revision+1 {
		t.Fatal("layout or card revision not updated")
	}
	b.request(t, "DELETE", "/groups/"+group.ID, nil, group.Revision, 409)
	b.request(t, "DELETE", "/groups/"+other.ID, nil, other.Revision, 204)
	b.request(t, "GET", "/missing-endpoint", nil, 0, 404)
	server.Close()
	if err := instance.Close(); err != nil {
		t.Fatal(err)
	}
	instance, server = start(t, cfg)
	b.base = server.URL
	b.request(t, "GET", "/me", nil, 0, 200)
	b.getCSRF(t)
	dashboard = load()
	if len(dashboard.Groups[0].Services) != 2 {
		t.Fatal("cards did not survive restart")
	}
	guest := newBrowser(server.URL)
	guest.getCSRF(t)
	guest.request(t, "POST", "/setup", credentials, 0, 409)
	b.request(t, "DELETE", "/sessions/current", nil, 0, 204)
	b.request(t, "GET", "/me", nil, 0, 401)
	b.request(t, "POST", "/sessions", map[string]string{"username": "owner", "password": "wrong-password"}, 0, 401)
	b.request(t, "POST", "/sessions", map[string]string{"username": "owner", "password": "correct horse battery staple"}, 0, 200)
	b.request(t, "GET", "/me", nil, 0, 200)
	card = dashboard.Groups[0].Services[1]
	b.request(t, "DELETE", "/services/"+card.ID, nil, card.Revision, 204)
	if len(load().Groups[0].Services) != 1 {
		t.Fatal("delete was not persisted")
	}
}

func TestConcurrentSetupHasOneAdministrator(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "concurrent.db"))
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	defer pool.Close()
	store := &storage.Repository{DB: db}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Go(func() { _, err := store.Setup(t.Context(), "owner", "already-hashed-test-value"); results <- err })
	}
	wg.Wait()
	close(results)
	created := 0
	for err := range results {
		if err == nil {
			created++
		} else if err != domain.ErrAlreadySetup {
			t.Fatal(err)
		}
	}
	if created != 1 {
		t.Fatalf("created %d administrators", created)
	}
	var foreignKeys int
	db.Raw("PRAGMA foreign_keys").Scan(&foreignKeys)
	var journalMode string
	db.Raw("PRAGMA journal_mode").Scan(&journalMode)
	var busyTimeout int
	db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout)
	if foreignKeys != 1 || strings.ToLower(journalMode) != "wal" || busyTimeout != 5000 {
		t.Fatalf("sqlite settings: foreign_keys=%d journal=%s busy_timeout=%d", foreignKeys, journalMode, busyTimeout)
	}
	if err := db.Create(&storage.ServiceModel{ID: "orphan", GroupID: "missing", Name: "orphan", URL: "https://example.com", Revision: 1}).Error; err == nil {
		t.Fatal("foreign key did not reject orphan card")
	}
}

func TestBodyLimitAndLoginRateLimit(t *testing.T) {
	instance, server := start(t, app.Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), SetupToken: setupToken})
	defer server.Close()
	defer instance.Close()
	b := newBrowser(server.URL)
	b.getCSRF(t)
	b.request(t, "POST", "/sessions", map[string]string{"username": "owner", "password": strings.Repeat("x", 70<<10)}, 0, 413)
	for i := 0; i < 9; i++ {
		b.request(t, "POST", "/sessions", map[string]string{}, 0, 400)
	}
	b.request(t, "POST", "/sessions", map[string]string{}, 0, 429)
}
