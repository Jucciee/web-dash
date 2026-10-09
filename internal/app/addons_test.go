package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"homedashboard/internal/app"
	"homedashboard/internal/domain"
)

func TestAddonAPIAuthenticationCSRFAndRestart(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer private-api-token" || r.Header.Get("Cookie") != "session=private-cookie" {
			t.Error("configured authentication did not reach the API")
		}
		if r.Header.Get("X-CSRF-Token") != "" {
			t.Error("dashboard credentials reached the upstream")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"count": 0, "echo": "private-api-token", "private-api-token/secret": "hidden"})
	}))
	defer upstream.Close()
	cfg := app.Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), SetupToken: setupToken}
	instance, server := start(t, cfg)
	defer func() { server.Close(); instance.Close() }()
	b := newBrowser(server.URL)
	b.request(t, "GET", "/addons", nil, 0, 401)
	b.getCSRF(t)
	b.request(t, "POST", "/addons/preview", map[string]any{}, 0, 401)
	b.request(t, "POST", "/setup", map[string]string{"setupToken": setupToken, "username": "owner", "password": "correct horse battery staple"}, 0, 201)
	input := domain.AddonWrite{Manifest: domain.AddonManifest{
		SchemaVersion: 1, Name: "내 API", Request: domain.AddonRequest{URL: upstream.URL, Headers: []domain.AddonHeader{{Name: "Authorization", Value: "Bearer {{token}}"}, {Name: "Cookie", Value: "{{cookie}}"}}},
		Inputs: []domain.AddonInput{{Key: "token", Type: "secret", Required: true}, {Key: "cookie", Type: "secret", Required: true}}, Fields: []domain.AddonField{{Label: "작업", Path: "/count", Format: "number"}},
	}, Secrets: map[string]string{"token": "private-api-token", "cookie": "session=private-cookie"}, Enabled: true}
	csrfToken := b.csrf
	b.csrf = ""
	b.request(t, "POST", "/addons", input, 0, 403)
	b.request(t, "POST", "/addons/preview", domain.AddonPreview{AddonWrite: input}, 0, 403)
	if calls.Load() != 0 {
		t.Fatal("rejected preview called upstream")
	}
	b.csrf = csrfToken
	previewRaw := b.request(t, "POST", "/addons/preview", domain.AddonPreview{AddonWrite: input}, 0, 200)
	preview := decode[domain.AddonSnapshot](t, previewRaw)
	if !preview.Available || preview.Values[0].Value != "0" || strings.Contains(string(previewRaw), "private-api-token") || strings.Contains(string(previewRaw), "private-cookie") {
		t.Fatalf("preview leaked authentication or lost zero: %s", previewRaw)
	}
	addon := decode[domain.Addon](t, b.request(t, "POST", "/addons", input, 0, 201))
	for _, path := range []string{"/addons", "/addons/" + addon.ID} {
		raw := b.request(t, "GET", path, nil, 0, 200)
		if strings.Contains(string(raw), "private-api-token") || strings.Contains(string(raw), "private-cookie") {
			t.Fatal("read API returned credentials")
		}
	}
	b.csrf = ""
	b.request(t, "POST", "/addons/"+addon.ID+"/read", nil, 0, 403)
	b.request(t, "PATCH", "/addons/"+addon.ID, input, addon.Revision, 403)
	b.request(t, "DELETE", "/addons/"+addon.ID, nil, addon.Revision, 403)
	b.csrf = csrfToken
	input.Manifest.Name, input.Secrets = "수정한 API", map[string]string{}
	b.request(t, "PATCH", "/addons/"+addon.ID, input, 0, 400)
	updated := decode[domain.Addon](t, b.request(t, "PATCH", "/addons/"+addon.ID, input, addon.Revision, 200))
	b.request(t, "PATCH", "/addons/"+addon.ID, input, addon.Revision, 409)
	b.request(t, "POST", "/addons/preview", map[string]any{"manifest": input.Manifest, "enabled": true, "script": "alert(1)"}, 0, 400)
	oversized := input
	oversized.Manifest.Request.Body = strings.Repeat("x", 65<<10)
	b.request(t, "POST", "/addons", oversized, 0, 413)
	key, err := os.ReadFile(filepath.Join(cfg.DataDir, "addon-secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	server.Close()
	instance.Close()
	instance, server = start(t, cfg)
	b.base = server.URL
	keyAfter, _ := os.ReadFile(filepath.Join(cfg.DataDir, "addon-secret.key"))
	if !bytes.Equal(key, keyAfter) {
		t.Fatal("encryption key changed during restart")
	}
	persisted := decode[domain.Addon](t, b.request(t, "GET", "/addons/"+addon.ID, nil, 0, 200))
	if persisted.Manifest.Name != "수정한 API" || len(persisted.ConfiguredSecrets) != 2 {
		t.Fatal("addon did not survive restart")
	}
	snapshot := decode[domain.AddonSnapshot](t, b.request(t, "POST", "/addons/"+addon.ID+"/read", nil, 0, 200))
	if !snapshot.Available || snapshot.Values[0].Value != "0" {
		t.Fatal("saved credentials were not restored")
	}
	input.Enabled = false
	disabled := decode[domain.Addon](t, b.request(t, "PATCH", "/addons/"+addon.ID, input, updated.Revision, 200))
	before := calls.Load()
	b.request(t, "POST", "/addons/"+addon.ID+"/read", nil, 0, 400)
	if calls.Load() != before {
		t.Fatal("disabled addon was fetched")
	}
	b.request(t, "DELETE", "/addons/"+addon.ID, nil, disabled.Revision, 204)
	b.request(t, "GET", "/addons/"+addon.ID, nil, 0, 404)
}
