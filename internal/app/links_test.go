package app_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"homedashboard/internal/app"
	"homedashboard/internal/domain"
)

func uploadIcon(t *testing.T, b *browser, content []byte, csrf bool, want int) string {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "icon.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(content)
	_ = w.Close()
	req, _ := http.NewRequest("POST", b.base+"/api/v1/icons", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if csrf {
		req.Header.Set("X-CSRF-Token", b.csrf)
	}
	response, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode != want {
		t.Fatalf("upload: want %d, got %d: %s", want, response.StatusCode, data)
	}
	if want != 201 {
		return ""
	}
	var envelope struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(data, &envelope)
	return envelope.Data.ID
}

func TestLinkIconsAndWebChecks(t *testing.T) {
	var visits atomic.Int32
	var imageData bytes.Buffer
	_ = png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 3, 3)))
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visits.Add(1)
		switch r.URL.Path {
		case "/icon.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageData.Bytes())
		case "/auth":
			w.WriteHeader(401)
		default:
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<link rel="icon" href="/icon.png">`))
		}
	}))
	defer target.Close()
	cfg := app.Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), SetupToken: setupToken}
	instance, server := start(t, cfg)
	t.Cleanup(func() { server.Close(); _ = instance.Close() })
	b := newBrowser(server.URL)
	b.getCSRF(t)
	b.request(t, "GET", "/icons/"+strings.Repeat("a", 64), nil, 0, 401)
	uploadIcon(t, b, imageData.Bytes(), true, 401)
	b.request(t, "POST", "/setup", map[string]string{"setupToken": setupToken, "username": "owner", "password": "correct horse battery staple"}, 0, 201)
	uploadIcon(t, b, imageData.Bytes(), false, 403)
	uploadIcon(t, b, []byte("<svg onload='alert(1)'/>"), true, 400)
	uploadIcon(t, b, make([]byte, (1<<20)+1), true, 413)
	asset := uploadIcon(t, b, imageData.Bytes(), true, 201)
	dashboards := decode[[]domain.Dashboard](t, b.request(t, "GET", "/dashboards", nil, 0, 200))
	dashboard := decode[domain.Dashboard](t, b.request(t, "GET", "/dashboards/"+dashboards[0].ID, nil, 0, 200))
	input := map[string]any{"groupId": dashboard.Groups[0].ID, "name": "일반 웹 링크", "url": target.URL + "/app", "iconMode": "site", "webCheckEnabled": true}
	link := decode[domain.Service](t, b.request(t, "POST", "/services", input, 0, 201))
	readImage := func(endpoint string) {
		t.Helper()
		response, err := b.client.Get(b.base + "/api/v1" + endpoint)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, _ := io.ReadAll(response.Body)
		if response.StatusCode != 200 || response.Header.Get("Content-Type") != "image/png" || !bytes.Equal(data, imageData.Bytes()) {
			t.Fatal("icon image was not served")
		}
		if response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("private icon must not enter a shared cache")
		}
	}
	readImage("/services/" + link.ID + "/icon")
	initial := decode[[]domain.WebStatus](t, b.request(t, "GET", "/dashboards/"+dashboard.ID+"/web-statuses", nil, 0, 200))
	if len(initial) != 1 || initial[0].State != "unknown" || initial[0].Checking {
		t.Fatal("reading statuses started a check")
	}
	b.request(t, "POST", "/dashboards/"+dashboard.ID+"/web-statuses/check", nil, 0, 200)
	awaitState := func(state string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			items := decode[[]domain.WebStatus](t, b.request(t, "GET", "/dashboards/"+dashboard.ID+"/web-statuses", nil, 0, 200))
			if len(items) == 1 && items[0].State == state {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("web status never became %s: %+v", state, items)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	awaitState("online")
	link = decode[domain.Service](t, b.request(t, "PATCH", "/services/"+link.ID, map[string]any{"url": target.URL + "/auth", "iconMode": "url", "iconUrl": target.URL + "/icon.png"}, link.Revision, 200))
	b.request(t, "POST", "/dashboards/"+dashboard.ID+"/web-statuses/check", nil, 0, 200)
	awaitState("auth")
	readImage("/services/" + link.ID + "/icon")
	link = decode[domain.Service](t, b.request(t, "PATCH", "/services/"+link.ID, map[string]any{"iconMode": "upload", "iconAssetId": asset, "webCheckEnabled": false}, link.Revision, 200))
	awaitState("disabled")
	readImage("/services/" + link.ID + "/icon")
	before := visits.Load()
	b.request(t, "POST", "/dashboards/"+dashboard.ID+"/web-statuses/check", nil, 0, 200)
	awaitState("disabled")
	if visits.Load() != before {
		t.Fatal("disabled link was checked")
	}
	b.request(t, "PATCH", "/services/"+link.ID, map[string]any{"iconAssetId": strings.Repeat("a", 64)}, link.Revision, 400)
	server.Close()
	if err := instance.Close(); err != nil {
		t.Fatal(err)
	}
	instance, server = start(t, cfg)
	b.base = server.URL
	dashboard = decode[domain.Dashboard](t, b.request(t, "GET", "/dashboards/"+dashboard.ID, nil, 0, 200))
	saved := dashboard.Groups[0].Services[0]
	if saved.IconMode != "upload" || saved.IconAssetID != asset || saved.WebCheckEnabled {
		t.Fatal("icon settings or explicit false did not survive restart")
	}
	readImage("/icons/" + asset)
	readImage("/services/" + link.ID + "/icon")
	// False also survives a new INSERT, rather than a database default turning it on.
	input["url"], input["webCheckEnabled"] = target.URL+"/bookmark", false
	created := decode[domain.Service](t, b.request(t, "POST", "/services", input, 0, 201))
	if created.WebCheckEnabled {
		t.Fatal("explicit false was lost on create")
	}
}
