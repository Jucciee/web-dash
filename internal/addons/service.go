package addons

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"homedashboard/internal/domain"
	"homedashboard/internal/services"
	"homedashboard/internal/storage"
	"homedashboard/internal/webfetch"
)

type Service struct {
	Store  *storage.Repository
	client *http.Client
	box    cipher.AEAD
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	cache  map[string]cached
	group  singleflight.Group
	limit  chan struct{}
}
type cached struct {
	revision int64
	expires  time.Time
	snapshot domain.AddonSnapshot
}

func New(store *storage.Repository, key []byte) (*Service, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	box, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	client := webfetch.NewClient()
	// Credentials belong to exactly the configured request. Never follow a
	// redirect, send browser cookies, or retain an upstream Set-Cookie.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{Store: store, client: client, box: box, ctx: ctx, cancel: cancel, cache: map[string]cached{}, limit: make(chan struct{}, 4)}, nil
}
func (s *Service) Close() { s.cancel(); s.client.CloseIdleConnections() }
func (s *Service) encrypt(values map[string]string) (string, error) {
	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, s.box.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(s.box.Seal(nonce, nonce, data, []byte("web-dash:addon:v1"))), nil
}
func (s *Service) decode(row storage.AddonModel) (domain.Addon, map[string]string, error) {
	addon := domain.Addon{ID: row.ID, Enabled: row.Enabled, Position: row.Position, Revision: row.Revision, Values: map[string]string{}, ConfiguredSecrets: []string{}}
	if err := json.Unmarshal([]byte(row.ManifestJSON), &addon.Manifest); err != nil {
		return addon, nil, errors.New("invalid stored addon manifest")
	}
	if err := json.Unmarshal([]byte(row.ValuesJSON), &addon.Values); err != nil {
		return addon, nil, errors.New("invalid stored addon values")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(row.SecretsCipher)
	if err != nil || len(sealed) < s.box.NonceSize() {
		return addon, nil, errors.New("invalid stored addon credentials")
	}
	plain, err := s.box.Open(nil, sealed[:s.box.NonceSize()], sealed[s.box.NonceSize():], []byte("web-dash:addon:v1"))
	if err != nil {
		return addon, nil, errors.New("cannot decrypt addon credentials; restore addon-secret.key")
	}
	secrets := map[string]string{}
	if err := json.Unmarshal(plain, &secrets); err != nil {
		return addon, nil, errors.New("invalid stored addon credentials")
	}
	for _, input := range addon.Manifest.Inputs {
		if input.Type == "secret" && secrets[input.Key] != "" {
			addon.ConfiguredSecrets = append(addon.ConfiguredSecrets, input.Key)
		}
	}
	return addon, secrets, nil
}
func (s *Service) List(ctx context.Context) ([]domain.Addon, error) {
	rows, err := s.Store.Addons(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Addon, 0, len(rows))
	for _, row := range rows {
		addon, _, err := s.decode(row)
		if err != nil {
			return nil, err
		}
		result = append(result, addon)
	}
	return result, nil
}
func (s *Service) Get(ctx context.Context, id string) (domain.Addon, error) {
	row, err := s.Store.Addon(ctx, id)
	if err != nil {
		return domain.Addon{}, err
	}
	addon, _, err := s.decode(row)
	return addon, err
}
func merged(public, secret map[string]string) map[string]string {
	values := map[string]string{}
	for k, v := range public {
		values[k] = v
	}
	for k, v := range secret {
		values[k] = v
	}
	return values
}
func origin(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Scheme + "://" + u.Host)
}

func (s *Service) prepare(ctx context.Context, id string, revision int64, input domain.AddonWrite, requireValues bool) (domain.AddonWrite, map[string]string, error) {
	m, err := ValidateManifest(input.Manifest)
	if err != nil {
		return input, nil, err
	}
	input.Manifest = m
	oldSecrets := map[string]string{}
	var old domain.Addon
	if id != "" {
		row, err := s.Store.Addon(ctx, id)
		if err != nil {
			return input, nil, err
		}
		if row.Revision != revision {
			return input, nil, domain.ErrConflict
		}
		old, oldSecrets, err = s.decode(row)
		if err != nil {
			return input, nil, err
		}
	}
	definitions := map[string]domain.AddonInput{}
	for _, definition := range m.Inputs {
		definitions[definition.Key] = definition
	}
	for key := range input.Values {
		if spec, ok := definitions[key]; !ok || spec.Type != "text" {
			return input, nil, invalid("일반 입력과 보안 입력을 정의한 항목에 맞춰 입력해주세요.")
		}
	}
	for key := range input.Secrets {
		if spec, ok := definitions[key]; !ok || spec.Type != "secret" {
			return input, nil, invalid("인증 값은 secret 입력 항목에만 저장할 수 있습니다.")
		}
	}
	values, secrets := map[string]string{}, map[string]string{}
	preserved := false
	for _, definition := range m.Inputs {
		value := ""
		if definition.Type == "secret" {
			var provided bool
			value, provided = input.Secrets[definition.Key]
			if !provided {
				for _, previous := range old.Manifest.Inputs {
					if previous.Key == definition.Key && previous.Type == "secret" {
						value = oldSecrets[definition.Key]
						preserved = preserved || value != ""
					}
				}
			}
			if value != "" {
				secrets[definition.Key] = value
			}
		} else {
			var provided bool
			value, provided = input.Values[definition.Key]
			if !provided {
				value = definition.Default
			}
			values[definition.Key] = value
		}
		if len(value) > 8192 || strings.ContainsRune(value, 0) {
			return input, nil, invalid("연결 입력값은 8KiB 이내로 입력해주세요.")
		}
		if requireValues && definition.Required && value == "" {
			return input, nil, invalid(definition.Label + " 값을 입력해주세요.")
		}
	}
	all := merged(values, secrets)
	address := render(m.Request.URL, all)
	if preserved && origin(address) != origin(render(old.Manifest.Request.URL, merged(old.Values, oldSecrets))) {
		return input, nil, invalid("API 서버 주소가 변경되었습니다. 인증 값을 다시 입력해주세요.")
	}
	if requireValues {
		if _, err := services.ValidateURL(address); err != nil {
			return input, nil, invalid("API 주소 또는 서버 주소 입력값을 확인해주세요. http·https만 사용할 수 있습니다.")
		}
		for _, header := range m.Request.Headers {
			value := render(header.Value, all)
			if len(value) > 16<<10 || strings.ContainsAny(value, "\r\n\x00") {
				return input, nil, invalid("헤더 연결값의 크기와 줄바꿈을 확인해주세요.")
			}
		}
		body := render(m.Request.Body, all)
		if len(body) > 64<<10 {
			return input, nil, invalid("입력값을 적용한 요청 본문은 64KiB까지 사용할 수 있습니다.")
		}
		if m.Request.Method == "POST" && body != "" && !json.Valid([]byte(body)) {
			return input, nil, invalid("POST 요청 본문을 유효한 JSON으로 입력해주세요. 문자열 입력은 {{key|json}}을 사용하세요.")
		}
	}
	input.Values, input.Secrets = values, secrets
	return input, all, nil
}
func (s *Service) Save(ctx context.Context, id string, revision int64, input domain.AddonWrite) (domain.Addon, error) {
	input, _, err := s.prepare(ctx, id, revision, input, input.Enabled)
	if err != nil {
		return domain.Addon{}, err
	}
	manifest, _ := json.Marshal(input.Manifest)
	values, _ := json.Marshal(input.Values)
	sealed, err := s.encrypt(input.Secrets)
	if err != nil {
		return domain.Addon{}, err
	}
	row, err := s.Store.SaveAddon(ctx, storage.AddonModel{ID: id, ManifestJSON: string(manifest), ValuesJSON: string(values), SecretsCipher: sealed, Enabled: input.Enabled}, revision)
	if err != nil {
		return domain.Addon{}, err
	}
	s.mu.Lock()
	delete(s.cache, row.ID)
	s.mu.Unlock()
	addon, _, err := s.decode(row)
	return addon, err
}
func (s *Service) Preview(ctx context.Context, input domain.AddonPreview) (domain.AddonSnapshot, error) {
	prepared, values, err := s.prepare(ctx, input.ID, input.Revision, input.AddonWrite, true)
	if err != nil {
		return domain.AddonSnapshot{}, err
	}
	bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return s.collect(bounded, prepared.Manifest, values, prepared.Secrets, true), nil
}
func (s *Service) Read(ctx context.Context, id string) (domain.AddonSnapshot, error) {
	row, err := s.Store.Addon(ctx, id)
	if err != nil {
		return domain.AddonSnapshot{}, err
	}
	if !row.Enabled {
		return domain.AddonSnapshot{}, invalid("비활성화된 애드온입니다.")
	}
	readCache := func() (domain.AddonSnapshot, bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		entry, ok := s.cache[id]
		return entry.snapshot, ok && entry.revision == row.Revision && time.Now().Before(entry.expires)
	}
	if value, fresh := readCache(); fresh {
		return value, nil
	}
	result := s.group.DoChan(id+":"+jsonRevision(row.Revision), func() (any, error) {
		if value, fresh := readCache(); fresh {
			return value, nil
		}
		addon, secrets, err := s.decode(row)
		if err != nil {
			return domain.AddonSnapshot{}, err
		}
		bounded, cancel := context.WithTimeout(s.ctx, 8*time.Second)
		defer cancel()
		value := s.collect(bounded, addon.Manifest, merged(addon.Values, secrets), secrets, false)
		s.mu.Lock()
		if len(s.cache) >= 32 {
			s.cache = map[string]cached{}
		}
		s.cache[id] = cached{revision: row.Revision, expires: time.Now().Add(5 * time.Second), snapshot: value}
		s.mu.Unlock()
		return value, nil
	})
	select {
	case <-ctx.Done():
		return domain.AddonSnapshot{}, ctx.Err()
	case result := <-result:
		if result.Err != nil {
			return domain.AddonSnapshot{}, result.Err
		}
		return result.Val.(domain.AddonSnapshot), nil
	}
}
func jsonRevision(revision int64) string { data, _ := json.Marshal(revision); return string(data) }
func (s *Service) Delete(ctx context.Context, id string, revision int64) error {
	if err := s.Store.DeleteAddon(ctx, id, revision); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()
	return nil
}
func redactor(secrets map[string]string) func(string) string {
	values := []string{}
	for _, value := range secrets {
		if value != "" {
			values = append(values, value, url.QueryEscape(value), base64.StdEncoding.EncodeToString([]byte(value)))
		}
	}
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	return func(text string) string {
		for _, value := range values {
			text = strings.ReplaceAll(text, value, "••••")
		}
		return text
	}
}
