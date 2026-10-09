package app_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"homedashboard/internal/app"
	"homedashboard/internal/domain"
)

func upload(t *testing.T, b *browser, endpoint, name string, content []byte, want int) string {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(content)
	_ = writer.Close()
	req, _ := http.NewRequest("POST", b.base+"/api/v1"+endpoint, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", b.csrf)
	response, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode != want {
		t.Fatalf("upload: %d, want %d: %s", response.StatusCode, want, data)
	}
	if want != 201 {
		return ""
	}
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result.Data.ID
}

func TestTelemetryAndBackgroundsRequireAuthenticationAndPersist(t *testing.T) {
	cfg := app.Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), SetupToken: setupToken, DockerHost: "invalid://engine", HostName: "home-host"}
	instance, server := start(t, cfg)
	defer func() { server.Close(); instance.Close() }()
	b := newBrowser(server.URL)
	for _, path := range []string{"/monitoring/docker", "/monitoring/host", "/backgrounds/" + string(bytes.Repeat([]byte{'a'}, 64))} {
		b.request(t, "GET", path, nil, 0, 401)
	}
	b.getCSRF(t)
	b.request(t, "POST", "/setup", map[string]string{"setupToken": setupToken, "username": "owner", "password": "correct horse battery staple"}, 0, 201)
	docker := decode[domain.DockerSnapshot](t, b.request(t, "GET", "/monitoring/docker", nil, 0, 200))
	if docker.Available || docker.Message == "" {
		t.Fatal("Docker connection failure must be explicit")
	}
	host := decode[domain.HostSnapshot](t, b.request(t, "GET", "/monitoring/host", nil, 0, 200))
	if !host.Available || host.Name != "home-host" || host.Memory == nil {
		t.Fatalf("real host metrics: %+v", host)
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	asset := upload(t, b, "/backgrounds", "background.png", imageData.Bytes(), http.StatusCreated)
	settings := decode[domain.Settings](t, b.request(t, "GET", "/settings", nil, 0, 200))
	appearance := settings.Appearance
	appearance.Background, appearance.BackgroundImage = "image", asset
	appearance.Columns, appearance.ShowClock, appearance.AccentColor = 6, false, "#12aa98"
	widgets := settings.Widgets
	widgets.ShowDocker, widgets.RefreshSeconds, widgets.Order = false, 0, []string{"docker", "host"}
	updated := decode[domain.Settings](t, b.request(t, "PATCH", "/settings", domain.SettingsPatch{Appearance: &appearance, Widgets: &widgets}, settings.Revision, 200))
	if updated.Appearance.ShowClock || updated.Widgets.ShowDocker || updated.Widgets.RefreshSeconds != 0 || updated.Appearance.BackgroundImage != asset {
		t.Fatal("zero/false customization not saved")
	}
	invalid := appearance
	invalid.AccentColor = "red; background: url(https://example.com)"
	b.request(t, "PATCH", "/settings", domain.SettingsPatch{Appearance: &invalid}, updated.Revision, 400)
	invalid = appearance
	invalid.Columns = 7
	b.request(t, "PATCH", "/settings", domain.SettingsPatch{Appearance: &invalid}, updated.Revision, 400)
	invalid = appearance
	invalid.BackgroundImage = string(bytes.Repeat([]byte{'a'}, 64))
	b.request(t, "PATCH", "/settings", domain.SettingsPatch{Appearance: &invalid}, updated.Revision, 400)
	invalidWidgets := widgets
	invalidWidgets.Order = []string{"host", "host"}
	b.request(t, "PATCH", "/settings", domain.SettingsPatch{Widgets: &invalidWidgets}, updated.Revision, 400)
	upload(t, b, "/backgrounds", "unsafe.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`), 400)
	server.Close()
	instance.Close()
	instance, server = start(t, cfg)
	b.base = server.URL
	persisted := decode[domain.Settings](t, b.request(t, "GET", "/settings", nil, 0, 200))
	if persisted.Appearance.BackgroundImage != asset || persisted.Appearance.ShowClock || persisted.Widgets.RefreshSeconds != 0 || persisted.Widgets.Order[0] != "docker" {
		t.Fatal("customization did not survive restart")
	}
	response, err := b.client.Get(server.URL + "/api/v1/backgrounds/" + asset)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "image/png" {
		t.Fatal("background did not survive restart")
	}
}
