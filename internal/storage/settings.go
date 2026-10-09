package storage

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
	"homedashboard/internal/domain"
)

func (r *Repository) Settings(ctx context.Context) (domain.Settings, error) {
	var row SettingsModel
	err := r.DB.WithContext(ctx).First(&row, "id = ?", 1).Error
	return toSettings(row), recordError(err)
}

func (r *Repository) UpdateSettings(ctx context.Context, revision int64, patch domain.SettingsPatch) (domain.Settings, error) {
	var row SettingsModel
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&row, "id = ?", 1).Error; err != nil {
			return recordError(err)
		}
		if row.Revision != revision {
			return domain.ErrConflict
		}
		if patch.Title != nil {
			row.Title = strings.TrimSpace(*patch.Title)
		}
		if patch.Subtitle != nil {
			row.Subtitle = strings.TrimSpace(*patch.Subtitle)
		}
		if patch.Theme != nil {
			row.Theme = *patch.Theme
		}
		if patch.Accent != nil {
			row.Accent = *patch.Accent
		}
		if patch.Compact != nil {
			row.Compact = *patch.Compact
		}
		if patch.CustomCSS != nil {
			row.CustomCSS = *patch.CustomCSS
		}
		if patch.Appearance != nil {
			if err := validateAppearance(*patch.Appearance); err != nil {
				return err
			}
			encoded, err := json.Marshal(patch.Appearance)
			if err != nil {
				return err
			}
			row.AppearanceJSON = string(encoded)
		}
		if patch.Widgets != nil {
			if err := validateWidgets(*patch.Widgets); err != nil {
				return err
			}
			encoded, err := json.Marshal(patch.Widgets)
			if err != nil {
				return err
			}
			row.WidgetsJSON = string(encoded)
		}
		if row.Title == "" || utf8.RuneCountInString(row.Title) > 80 || utf8.RuneCountInString(row.Subtitle) > 200 {
			return &domain.ValidationError{Message: "제목은 1~80자, 설명은 200자까지 입력해주세요."}
		}
		if len(row.CustomCSS) > 32<<10 || strings.ContainsRune(row.CustomCSS, 0) {
			return &domain.ValidationError{Message: "사용자 CSS는 UTF-8 기준 32KiB까지 입력할 수 있습니다."}
		}
		switch row.Theme {
		case "light", "dark", "system":
		default:
			return &domain.ValidationError{Message: "테마를 목록에서 선택해주세요."}
		}
		switch row.Accent {
		case "blue", "green", "amber", "slate", "violet", "rose", "custom":
		default:
			return &domain.ValidationError{Message: "강조색을 목록에서 선택해주세요."}
		}
		result := tx.Model(&SettingsModel{}).Where("id = 1 AND revision = ?", revision).Updates(map[string]any{
			"title": row.Title, "subtitle": row.Subtitle, "theme": row.Theme, "accent": row.Accent, "compact": row.Compact, "custom_css": row.CustomCSS, "revision": revision + 1,
			"appearance_json": row.AppearanceJSON, "widgets_json": row.WidgetsJSON,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		row.Revision++
		return nil
	})
	return toSettings(row), err
}

func toSettings(row SettingsModel) domain.Settings {
	appearance, widgets := domain.DefaultAppearance(), domain.DefaultWidgets()
	_ = json.Unmarshal([]byte(row.AppearanceJSON), &appearance)
	_ = json.Unmarshal([]byte(row.WidgetsJSON), &widgets)
	return domain.Settings{Title: row.Title, Subtitle: row.Subtitle, Theme: row.Theme, Accent: row.Accent, Compact: row.Compact, CustomCSS: row.CustomCSS, Appearance: appearance, Widgets: widgets, Revision: row.Revision}
}

var colorPattern = regexp.MustCompile(`^#[a-fA-F0-9]{6}$`)
var assetPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validateAppearance(a domain.Appearance) error {
	choices := []struct {
		value   string
		allowed []string
	}{
		{a.Layout, []string{"grid", "rows"}}, {a.Width, []string{"narrow", "normal", "wide", "full"}},
		{a.CardStyle, []string{"card", "tile", "list"}}, {a.IconSize, []string{"small", "medium", "large"}},
		{a.Radius, []string{"square", "soft", "round"}}, {a.FontSize, []string{"small", "normal", "large"}},
		{a.FontFamily, []string{"noto", "system", "mono"}}, {a.Background, []string{"plain", "grid", "dots", "image"}},
	}
	for _, choice := range choices {
		found := false
		for _, allowed := range choice.allowed {
			if choice.value == allowed {
				found = true
				break
			}
		}
		if !found {
			return &domain.ValidationError{Message: "화면 설정 값을 목록에서 선택해주세요."}
		}
	}
	if a.Columns < 0 || a.Columns > 6 || a.BackgroundDim < 0 || a.BackgroundDim > 90 {
		return &domain.ValidationError{Message: "열 수는 자동 또는 1~6, 배경 밝기 조절은 0~90 범위로 설정해주세요."}
	}
	if a.AccentColor != "" && !colorPattern.MatchString(a.AccentColor) {
		return &domain.ValidationError{Message: "사용자 강조색은 #RRGGBB 형식으로 입력해주세요."}
	}
	if a.BackgroundImage != "" && !assetPattern.MatchString(a.BackgroundImage) {
		return &domain.ValidationError{Message: "배경 이미지를 다시 선택해주세요."}
	}
	if a.Background == "image" && a.BackgroundImage == "" {
		return &domain.ValidationError{Message: "배경으로 사용할 이미지를 선택해주세요."}
	}
	return nil
}

func validateWidgets(w domain.Widgets) error {
	if w.RefreshSeconds != 0 && w.RefreshSeconds != 10 && w.RefreshSeconds != 30 && w.RefreshSeconds != 60 {
		return &domain.ValidationError{Message: "새로고침 간격을 목록에서 선택해주세요."}
	}
	if len(w.Order) != 2 || !((w.Order[0] == "host" && w.Order[1] == "docker") || (w.Order[0] == "docker" && w.Order[1] == "host")) {
		return &domain.ValidationError{Message: "위젯 순서에는 호스트와 Docker를 한 번씩 포함해주세요."}
	}
	return nil
}
