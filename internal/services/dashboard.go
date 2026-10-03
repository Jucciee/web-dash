package services

import (
	"context"
	"encoding/hex"
	"net/url"
	"strings"
	"unicode/utf8"

	"homedashboard/internal/domain"
	"homedashboard/internal/storage"
)

type Dashboard struct {
	Store             *storage.Repository
	ValidateIconAsset func(string) error
}

func ValidateURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.ContainsAny(raw, "\r\n\t ") || strings.Contains(u.Host, "\\") {
		return "", &domain.ValidationError{Message: "로그인 정보가 없는 http 또는 https 주소를 입력해주세요."}
	}
	return raw, nil
}

func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 80 {
		return "", &domain.ValidationError{Message: "이름을 1~80자로 입력해주세요."}
	}
	return name, nil
}

func ValidateService(input domain.ServiceInput) (domain.ServiceInput, error) {
	var err error
	input.Name, err = ValidateName(input.Name)
	if err != nil {
		return input, err
	}
	input.URL, err = ValidateURL(input.URL)
	if err != nil {
		return input, err
	}
	switch input.IconKey {
	case "", "server", "photos", "media", "files", "code", "home", "network", "database":
	default:
		return input, &domain.ValidationError{Message: "목록에서 기본 아이콘을 선택해주세요."}
	}
	if input.IconMode == "" {
		input.IconMode = "builtin"
	}
	switch input.IconMode {
	case "builtin", "site":
		input.IconURL, input.IconAssetID = "", ""
	case "url":
		input.IconURL, err = ValidateURL(input.IconURL)
		if err != nil {
			return input, &domain.ValidationError{Message: "아이콘 이미지의 http 또는 https 주소를 입력해주세요."}
		}
		input.IconAssetID = ""
	case "upload":
		data, err := hex.DecodeString(input.IconAssetID)
		if err != nil || len(data) != 32 || strings.ToLower(input.IconAssetID) != input.IconAssetID {
			return input, &domain.ValidationError{Message: "아이콘 파일을 먼저 업로드해주세요."}
		}
		input.IconURL = ""
	default:
		return input, &domain.ValidationError{Message: "아이콘 표시 방식을 선택해주세요."}
	}
	input.Description = strings.TrimSpace(input.Description)
	if utf8.RuneCountInString(input.Description) > 500 {
		return input, &domain.ValidationError{Message: "설명은 500자까지 입력할 수 있습니다."}
	}
	return input, nil
}

func (s *Dashboard) Create(ctx context.Context, input domain.ServiceInput) (domain.Service, error) {
	input, err := ValidateService(input)
	if err != nil {
		return domain.Service{}, err
	}
	if input.IconMode == "upload" && s.ValidateIconAsset != nil {
		if err := s.ValidateIconAsset(input.IconAssetID); err != nil {
			return domain.Service{}, err
		}
	}
	return s.Store.CreateService(ctx, input)
}

func (s *Dashboard) Update(ctx context.Context, id string, revision int64, patch domain.ServicePatch) (domain.Service, error) {
	old, err := s.Store.Service(ctx, id)
	if err != nil {
		return domain.Service{}, err
	}
	if old.Revision != revision {
		return domain.Service{}, domain.ErrConflict
	}
	input := domain.ServiceInput{GroupID: old.GroupID, Name: old.Name, URL: old.URL, IconKey: old.IconKey, IconMode: old.IconMode, IconURL: old.IconURL, IconAssetID: old.IconAssetID, WebCheckEnabled: &old.WebCheckEnabled, Description: old.Description, Hidden: old.Hidden}
	if patch.GroupID != nil {
		input.GroupID = *patch.GroupID
	}
	if patch.Name != nil {
		input.Name = *patch.Name
	}
	if patch.URL != nil {
		input.URL = *patch.URL
	}
	if patch.IconKey != nil {
		input.IconKey = *patch.IconKey
	}
	if patch.IconMode != nil {
		input.IconMode = *patch.IconMode
	}
	if patch.IconURL != nil {
		input.IconURL = *patch.IconURL
	}
	if patch.IconAssetID != nil {
		input.IconAssetID = *patch.IconAssetID
	}
	if patch.WebCheckEnabled != nil {
		input.WebCheckEnabled = patch.WebCheckEnabled
	}
	if patch.Description != nil {
		input.Description = *patch.Description
	}
	if patch.Hidden != nil {
		input.Hidden = *patch.Hidden
	}
	input, err = ValidateService(input)
	if err != nil {
		return domain.Service{}, err
	}
	if input.IconMode == "upload" && s.ValidateIconAsset != nil {
		if err := s.ValidateIconAsset(input.IconAssetID); err != nil {
			return domain.Service{}, err
		}
	}
	return s.Store.UpdateService(ctx, id, revision, input)
}
