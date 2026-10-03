package icons

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"homedashboard/internal/domain"
	"homedashboard/internal/webfetch"
)

func testPNG(t *testing.T, width int) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewRGBA(image.Rect(0, 0, width, 1))); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestUploadsValidationAndPersistence(t *testing.T) {
	client := webfetch.NewClient()
	defer client.CloseIdleConnections()
	store, err := New(t.TempDir(), client)
	if err != nil {
		t.Fatal(err)
	}
	data := testPNG(t, 1)
	id, err := store.Save(data)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate, err := store.Save(data); err != nil || duplicate != id {
		t.Fatal("identical uploads should share immutable data")
	}
	reopened, err := New(store.Dir, client)
	if err != nil {
		t.Fatal(err)
	}
	img, err := reopened.Uploaded(id)
	if err != nil || img.MIME != "image/png" || !bytes.Equal(img.Data, data) {
		t.Fatal("uploaded image did not persist")
	}
	for _, invalid := range [][]byte{nil, []byte("<svg onload='alert(1)'/>"), []byte("not an image"), make([]byte, MaxUpload+1), testPNG(t, 2049)} {
		if _, err := store.Save(invalid); err == nil {
			t.Fatal("invalid upload was accepted")
		}
	}
	if _, err := store.Uploaded("../../dashboard.db"); err == nil {
		t.Fatal("invalid image ID accepted")
	}
	ico := make([]byte, 22)
	ico[2], ico[4], ico[6], ico[7] = 1, 1, 1, 1
	binary.LittleEndian.PutUint32(ico[14:18], uint32(len(data)))
	binary.LittleEndian.PutUint32(ico[18:22], 22)
	ico = append(ico, data...)
	if mime, err := Validate(ico); err != nil || mime != "image/x-icon" {
		t.Fatal("valid ICO rejected")
	}
	ico[18] = 255
	if _, err := Validate(ico); err == nil {
		t.Fatal("out-of-bounds ICO accepted")
	}
}

func TestSiteDiscoveryCustomURLFallbackAndCaching(t *testing.T) {
	data := testPNG(t, 2)
	var iconReads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><link rel="shortcut icon" href="/assets/icon.png"></html>`))
		case "/without-link":
			_, _ = w.Write([]byte("<html><title>No icon link</title></html>"))
		case "/assets/icon.png", "/favicon.ico":
			iconReads.Add(1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(data)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := webfetch.NewClient()
	defer client.CloseIdleConnections()
	store, err := New(t.TempDir(), client)
	if err != nil {
		t.Fatal(err)
	}
	link := domain.Service{IconMode: "site", URL: server.URL + "/app"}
	for range 3 {
		if img := store.ForLink(t.Context(), link); img.MIME != "image/png" || !bytes.Equal(img.Data, data) {
			t.Fatal("relative declared icon was not discovered")
		}
	}
	if iconReads.Load() != 1 {
		t.Fatal("remote icon was not cached")
	}
	link.URL = server.URL + "/without-link"
	if img := store.ForLink(t.Context(), link); !bytes.Equal(img.Data, data) {
		t.Fatal("favicon.ico fallback failed")
	}
	link.IconMode, link.IconURL = "url", server.URL+"/assets/icon.png"
	if img := store.ForLink(t.Context(), link); !bytes.Equal(img.Data, data) {
		t.Fatal("custom image URL failed")
	}
	link.IconURL = server.URL + "/missing"
	if img := store.ForLink(t.Context(), link); len(img.Data) != 0 {
		t.Fatal("missing icon should use built-in fallback")
	}
}
