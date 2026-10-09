package backgrounds

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"

	_ "golang.org/x/image/webp"
	"homedashboard/internal/domain"
)

const MaxUpload = 8 << 20

var idPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Store struct{ dir string }

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func validate(data []byte) (string, error) {
	if len(data) == 0 || len(data) > MaxUpload {
		return "", &domain.ValidationError{Message: "배경 이미지는 8MiB까지 업로드할 수 있습니다."}
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 6000 || config.Height > 6000 || config.Width*config.Height > 24_000_000 {
		return "", &domain.ValidationError{Message: "최대 6000px·2400만 화소의 PNG, JPG, WebP 이미지를 선택해주세요."}
	}
	switch format {
	case "png":
		return "image/png", nil
	case "jpeg":
		return "image/jpeg", nil
	case "webp":
		return "image/webp", nil
	default:
		return "", &domain.ValidationError{Message: "PNG, JPG, WebP 이미지를 선택해주세요."}
	}
}

func (s *Store) Save(data []byte) (string, error) {
	if _, err := validate(data); err != nil {
		return "", err
	}
	id := fmt.Sprintf("%x", sha256.Sum256(data))
	target := filepath.Join(s.dir, id)
	if _, err := os.Stat(target); err == nil {
		return id, nil
	}
	tmp, err := os.CreateTemp(s.dir, "upload-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		if _, statErr := os.Stat(target); statErr != nil {
			return "", err
		}
	}
	return id, nil
}

func (s *Store) Read(id string) ([]byte, string, error) {
	if !idPattern.MatchString(id) {
		return nil, "", domain.ErrNotFound
	}
	data, err := os.ReadFile(filepath.Join(s.dir, id))
	if os.IsNotExist(err) {
		return nil, "", domain.ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	mime, err := validate(data)
	return data, mime, err
}
