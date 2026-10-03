package icons

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/webp"
	"golang.org/x/net/html"
	"golang.org/x/sync/singleflight"
	"homedashboard/internal/domain"
	"homedashboard/internal/webfetch"
)

const MaxUpload = 1 << 20
const maxRemote = 256 << 10

type Image struct {
	Data []byte
	MIME string
}
type cached struct {
	Image
	until time.Time
}
type Store struct {
	Dir     string
	Client  *http.Client
	mu      sync.Mutex
	cache   map[string]cached
	flights singleflight.Group
	slots   chan struct{}
}

func New(dir string, client *http.Client) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Store{Dir: dir, Client: client, cache: map[string]cached{}, slots: make(chan struct{}, 4)}, nil
}

func Validate(data []byte) (string, error) {
	invalid := &domain.ValidationError{Message: "1MB 이하, 가로·세로 2048px 이하의 PNG, JPG, WebP 또는 ICO 파일을 선택해주세요."}
	if len(data) == 0 || len(data) > MaxUpload {
		return "", invalid
	}
	if len(data) >= 6 && bytes.Equal(data[:4], []byte{0, 0, 1, 0}) {
		count := int(binary.LittleEndian.Uint16(data[4:6]))
		if count < 1 || count > 256 || len(data) < 6+16*count {
			return "", invalid
		}
		for i := range count {
			entry := data[6+i*16 : 6+(i+1)*16]
			size, offset := uint64(binary.LittleEndian.Uint32(entry[8:12])), uint64(binary.LittleEndian.Uint32(entry[12:16]))
			if size == 0 || offset < uint64(6+16*count) || offset+size > uint64(len(data)) {
				return "", invalid
			}
		}
		return "image/x-icon", nil
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 2048 || config.Height > 2048 {
		return "", invalid
	}
	switch format {
	case "png":
		return "image/png", nil
	case "jpeg":
		return "image/jpeg", nil
	case "webp":
		return "image/webp", nil
	}
	return "", invalid
}

func validID(id string) bool {
	raw, err := hex.DecodeString(id)
	return err == nil && len(raw) == 32 && id == strings.ToLower(id)
}

func (s *Store) Save(data []byte) (string, error) {
	if _, err := Validate(data); err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	id := hex.EncodeToString(hash[:])
	file, err := os.OpenFile(filepath.Join(s.Dir, id), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return id, nil
	}
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(filepath.Join(s.Dir, id))
		return "", writeErr
	}
	return id, closeErr
}

func (s *Store) Uploaded(id string) (Image, error) {
	if !validID(id) {
		return Image{}, domain.ErrNotFound
	}
	file, err := os.Open(filepath.Join(s.Dir, id))
	if os.IsNotExist(err) {
		return Image{}, domain.ErrNotFound
	}
	if err != nil {
		return Image{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxUpload+1))
	if err != nil {
		return Image{}, err
	}
	mime, err := Validate(data)
	if err != nil {
		return Image{}, err
	}
	return Image{data, mime}, nil
}

func (s *Store) ForLink(ctx context.Context, link domain.Service) Image {
	if link.IconMode == "upload" {
		img, _ := s.Uploaded(link.IconAssetID)
		return img
	}
	if link.IconMode != "site" && link.IconMode != "url" {
		return Image{}
	}
	key := link.IconMode + "\n" + link.URL + "\n" + link.IconURL
	s.mu.Lock()
	entry, found := s.cache[key]
	s.mu.Unlock()
	if found && time.Now().Before(entry.until) {
		return entry.Image
	}
	result := s.flights.DoChan(key, func() (any, error) {
		fetchCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		defer cancel()
		s.mu.Lock()
		entry, found := s.cache[key]
		s.mu.Unlock()
		if found && time.Now().Before(entry.until) {
			return entry.Image, nil
		}
		select {
		case s.slots <- struct{}{}:
			defer func() { <-s.slots }()
		case <-fetchCtx.Done():
			return Image{}, nil
		}
		img := Image{}
		if link.IconMode == "url" {
			img = s.fetchImage(fetchCtx, link.IconURL)
		} else {
			img = s.siteIcon(fetchCtx, link.URL)
		}
		ttl := time.Hour
		if len(img.Data) == 0 {
			ttl = 5 * time.Minute
		}
		s.mu.Lock()
		if len(s.cache) >= 64 {
			for k := range s.cache {
				delete(s.cache, k)
				break
			}
		}
		s.cache[key] = cached{img, time.Now().Add(ttl)}
		s.mu.Unlock()
		return img, nil
	})
	select {
	case result := <-result:
		if result.Err == nil {
			return result.Val.(Image)
		}
	case <-ctx.Done():
	}
	return Image{}
}

func (s *Store) fetchImage(ctx context.Context, address string) Image {
	response, err := webfetch.Request(ctx, s.Client, http.MethodGet, address)
	if err != nil {
		return Image{}
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return Image{}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxRemote+1))
	if err != nil || len(data) > maxRemote {
		return Image{}
	}
	mime, err := Validate(data)
	if err != nil {
		return Image{}
	}
	return Image{data, mime}
}

func (s *Store) siteIcon(ctx context.Context, address string) Image {
	base, err := url.Parse(address)
	if err != nil {
		return Image{}
	}
	response, err := webfetch.Request(ctx, s.Client, http.MethodGet, address)
	if err == nil {
		base = response.Request.URL
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			data, readErr := io.ReadAll(io.LimitReader(response.Body, maxRemote+1))
			response.Body.Close()
			if readErr == nil && len(data) <= maxRemote {
				if img := s.htmlIcon(ctx, data, base); len(img.Data) > 0 {
					return img
				}
			}
		} else {
			response.Body.Close()
		}
	}
	fallback := *base
	fallback.Path, fallback.RawPath, fallback.RawQuery, fallback.Fragment = "/favicon.ico", "", "", ""
	return s.fetchImage(ctx, fallback.String())
}

func (s *Store) htmlIcon(ctx context.Context, data []byte, base *url.URL) Image {
	z := html.NewTokenizer(bytes.NewReader(data))
	addresses := []string{}
	for len(addresses) < 8 {
		switch z.Next() {
		case html.ErrorToken:
			for _, address := range addresses {
				if img := s.fetchImage(ctx, address); len(img.Data) > 0 {
					return img
				}
			}
			return Image{}
		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			if token.Data != "link" {
				continue
			}
			rel, href := "", ""
			for _, attr := range token.Attr {
				if attr.Key == "rel" {
					rel = strings.ToLower(attr.Val)
				}
				if attr.Key == "href" {
					href = attr.Val
				}
			}
			isIcon := false
			for _, value := range strings.Fields(rel) {
				if value == "icon" || value == "apple-touch-icon" {
					isIcon = true
				}
			}
			if isIcon {
				ref, err := url.Parse(href)
				if err == nil && href != "" {
					addresses = append(addresses, base.ResolveReference(ref).String())
				}
			}
		}
	}
	for _, address := range addresses {
		if img := s.fetchImage(ctx, address); len(img.Data) > 0 {
			return img
		}
	}
	return Image{}
}
