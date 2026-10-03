package app_test

import (
	"io"
	"net/http"
	"testing"

	"homedashboard/internal/app"
	"homedashboard/internal/domain"
)

func TestSettingsRequireAuthPersistAndRejectStaleRevision(t *testing.T) {
	cfg := app.Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), SetupToken: setupToken}
	instance, server := start(t, cfg)
	b := newBrowser(server.URL)
	b.request(t, "GET", "/settings", nil, 0, 401)
	b.getCSRF(t)
	b.request(t, "POST", "/setup", map[string]string{"setupToken": setupToken, "username": "owner", "password": "correct horse battery staple"}, 0, 201)
	settings := decode[domain.Settings](t, b.request(t, "GET", "/settings", nil, 0, 200))
	patch := map[string]any{"theme": "dark", "compact": true, "title": "나의 서비스", "customCss": ".service-card { border-radius: 20px; }"}
	updated := decode[domain.Settings](t, b.request(t, "PATCH", "/settings", patch, settings.Revision, 200))
	if updated.Theme != "dark" || !updated.Compact || updated.Revision != 2 {
		t.Fatalf("settings not updated: %+v", updated)
	}
	b.request(t, "PATCH", "/settings", patch, settings.Revision, 409)
	b.request(t, "PATCH", "/settings", map[string]string{"theme": "unknown"}, updated.Revision, 400)
	updated = decode[domain.Settings](t, b.request(t, "PATCH", "/settings", map[string]any{"compact": false}, updated.Revision, 200))
	if updated.Compact {
		t.Fatal("false settings patch ignored")
	}
	response, err := b.client.Get(server.URL + "/api/v1/settings/custom.css")
	if err != nil {
		t.Fatal(err)
	}
	css, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || string(css) != patch["customCss"] || response.Header.Get("Content-Type") != "text/css; charset=utf-8" || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("custom stylesheet: %s, %s", response.Status, css)
	}
	server.Close()
	instance.Close()
	instance, server = start(t, cfg)
	defer instance.Close()
	defer server.Close()
	b.base = server.URL
	persisted := decode[domain.Settings](t, b.request(t, "GET", "/settings", nil, 0, 200))
	if persisted.Title != updated.Title || persisted.CustomCSS != updated.CustomCSS {
		t.Fatal("settings did not survive server restart")
	}
}
