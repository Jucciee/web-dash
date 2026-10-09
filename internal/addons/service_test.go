package addons

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"homedashboard/internal/domain"
	"homedashboard/internal/storage"
)

func fixtureManifest(address string) domain.AddonManifest {
	return domain.AddonManifest{SchemaVersion: 1, Name: "NAS API", RefreshSeconds: 0, Request: domain.AddonRequest{Method: "GET", URL: address, Headers: []domain.AddonHeader{{Name: "Authorization", Value: "Bearer {{token}}"}, {Name: "Cookie", Value: "{{cookie}}"}}}, Inputs: []domain.AddonInput{{Key: "token", Label: "토큰", Type: "secret", Required: true}, {Key: "cookie", Label: "쿠키", Type: "secret", Required: true}}, Fields: []domain.AddonField{{Label: "작업", Path: "/metrics/active", Format: "number"}, {Label: "부하", Path: "/metrics/load", Format: "percent", Scale: 100, Decimals: 1}, {Label: "목록", Path: "/items", Format: "count"}, {Label: "상태", Path: "/status", Format: "text", ValueMap: map[string]string{"healthy": "정상"}}, {Label: "없는 값", Path: "/missing", Format: "text"}}}
}
func TestCredentialsPersistenceRedactionCacheAndOrdering(t *testing.T) {
	var calls atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-secret-token" || r.Header.Get("Cookie") != "session=test-secret-cookie" || r.Header.Get("X-CSRF-Token") != "" {
			t.Error("credentials were not rendered or browser headers leaked")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"metrics":{"active":0,"load":0.25},"items":[1,2],"status":"healthy","echo":"test-secret-token","cookie":"session=test-secret-cookie"}`))
	}))
	defer api.Close()
	db, err := storage.Open(filepath.Join(t.TempDir(), "addons.db"))
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	defer pool.Close()
	store := &storage.Repository{DB: db}
	key := bytes.Repeat([]byte{7}, 32)
	s, err := New(store, key)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	input := domain.AddonWrite{Manifest: fixtureManifest(api.URL), Enabled: true, Secrets: map[string]string{"token": "test-secret-token", "cookie": "session=test-secret-cookie"}}
	addon, err := s.Save(t.Context(), "", 0, input)
	if err != nil {
		t.Fatal(err)
	}
	row, _ := store.Addon(t.Context(), addon.ID)
	if strings.Contains(row.SecretsCipher, "test-secret") || strings.Contains(row.ManifestJSON, "test-secret") || strings.Contains(row.ValuesJSON, "test-secret") {
		t.Fatal("plaintext secret in database")
	}
	public, _ := json.Marshal(addon)
	if strings.Contains(string(public), "test-secret") || len(addon.ConfiguredSecrets) != 2 {
		t.Fatal("secret returned to client")
	}
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			snapshot, err := s.Read(t.Context(), addon.ID)
			if err != nil || !snapshot.Available || snapshot.Values[0].Value != "0" || snapshot.Values[1].Value != "25.0%" || snapshot.Values[2].Value != "2" || snapshot.Values[3].Value != "정상" || !snapshot.Values[4].Missing {
				t.Errorf("snapshot: %+v %v", snapshot, err)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("shared reads: %d", calls.Load())
	}
	preview, err := s.Preview(t.Context(), domain.AddonPreview{ID: addon.ID, Revision: addon.Revision, AddonWrite: domain.AddonWrite{Manifest: addon.Manifest, Values: addon.Values, Enabled: true}})
	if err != nil || !preview.Available {
		t.Fatal(preview, err)
	}
	data, _ := json.Marshal(preview)
	if strings.Contains(string(data), "test-secret") {
		t.Fatal("secret echoed in preview")
	}
	input.Manifest.Name, input.Secrets = "수정한 NAS", map[string]string{}
	updated, err := s.Save(t.Context(), addon.ID, addon.Revision, input)
	if err != nil || len(updated.ConfiguredSecrets) != 2 {
		t.Fatal("editing lost saved credentials", err)
	}
	if _, err := s.Save(t.Context(), addon.ID, addon.Revision, input); err != domain.ErrConflict {
		t.Fatal("stale revision accepted", err)
	}
	input.Manifest.Request.URL = "http://example.com/new-api"
	if _, err := s.Save(t.Context(), addon.ID, updated.Revision, input); err == nil {
		t.Fatal("saved secrets reused across origins")
	}
	s.Close()
	s, err = New(store, key)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if result, err := s.Read(t.Context(), addon.ID); err != nil || !result.Available {
		t.Fatal("restart lost credentials", err)
	}
	input.Manifest.Request.URL, input.Manifest.Name, input.Enabled = api.URL, "두 번째", false
	other, err := s.Save(t.Context(), "", 0, input)
	if err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	if _, err := s.Read(t.Context(), other.ID); err == nil || calls.Load() != before {
		t.Fatal("disabled addon called API")
	}
	if err := store.MoveAddon(t.Context(), other.ID, other.Revision, -1); err != nil {
		t.Fatal(err)
	}
	list, _ := s.List(t.Context())
	if list[0].ID != other.ID {
		t.Fatal("widget order not saved")
	}
	if err := s.Delete(t.Context(), list[0].ID, list[0].Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(t.Context(), other.ID); err != domain.ErrNotFound {
		t.Fatal("uninstall failed", err)
	}
}

func TestPOSTTemplatesNoRedirectsAndBoundedResponses(t *testing.T) {
	var redirected atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1); w.WriteHeader(200) }))
	defer other.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, other.URL, http.StatusFound)
		case "/large":
			_, _ = w.Write(bytes.Repeat([]byte{'x'}, (1<<20)+1))
		case "/html":
			_, _ = w.Write([]byte("<html/>"))
		default:
			if r.Method != "POST" || r.URL.Query().Get("filter") != "a & b" || r.Header.Get("Authorization") != "Basic dXNlcjpwYXNz" {
				t.Error("request templates were not applied")
			}
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["query"] != "quote\"newline\n" {
				t.Error("JSON interpolation was not escaped")
			}
			_, _ = w.Write([]byte(`{"a/b":{"~key":[false,0]}}`))
		}
	}))
	defer api.Close()
	s, _ := New(nil, bytes.Repeat([]byte{1}, 32))
	defer s.Close()
	m := domain.AddonManifest{SchemaVersion: 1, Name: "POST", Request: domain.AddonRequest{Method: "POST", URL: api.URL + "?filter={{filter|url}}", Headers: []domain.AddonHeader{{Name: "Authorization", Value: "Basic {{credentials|base64}}"}}, Body: `{"query":{{query|json}}}`}, Inputs: []domain.AddonInput{{Key: "filter", Type: "text"}, {Key: "query", Type: "text"}, {Key: "credentials", Type: "secret"}}, Fields: []domain.AddonField{{Label: "false", Path: "/a~1b/~0key/0", Format: "boolean"}, {Label: "zero", Path: "/a~1b/~0key/1", Format: "number"}}}
	input := domain.AddonPreview{AddonWrite: domain.AddonWrite{Manifest: m, Values: map[string]string{"filter": "a & b", "query": "quote\"newline\n"}, Secrets: map[string]string{"credentials": "user:pass"}}}
	result, err := s.Preview(t.Context(), input)
	if err != nil || !result.Available || result.Values[0].Value != "아니요" || result.Values[1].Value != "0" {
		t.Fatal(result, err)
	}
	for _, path := range []string{"/redirect", "/large", "/html"} {
		input.Manifest.Request.URL = api.URL + path
		result, err := s.Preview(t.Context(), input)
		if err != nil || result.Available || result.Message == "" {
			t.Fatal(path, result, err)
		}
	}
	if redirected.Load() != 0 {
		t.Fatal("redirect forwarded credentials")
	}
	input.Manifest.Request.URL = "http://169.254.169.254/metadata"
	result, err = s.Preview(t.Context(), input)
	if err != nil || result.Available {
		t.Fatal("link-local endpoint allowed")
	}
	input.Manifest.Request.URL = api.URL
	input.Secrets["credentials"] = "user\n:pass"
	// Base64 credentials are safe header text. A raw-cookie template is not.
	input.Manifest.Request.Headers[0] = domain.AddonHeader{Name: "Cookie", Value: "{{credentials}}"}
	if _, err := s.Preview(t.Context(), input); err == nil {
		t.Fatal("header newline injection accepted")
	}
	input.Manifest.Request.Headers[0] = domain.AddonHeader{Name: "Authorization", Value: "literal-token"}
	if _, err := ValidateManifest(input.Manifest); err == nil {
		t.Fatal("literal credential accepted in shareable manifest")
	}
	input.Manifest.Request.Headers[0] = domain.AddonHeader{Name: "Host", Value: "elsewhere"}
	if _, err := ValidateManifest(input.Manifest); err == nil {
		t.Fatal("Host override accepted")
	}
}
