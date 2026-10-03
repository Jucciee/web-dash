package storage

import (
	"context"
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
		case "blue", "green", "amber", "slate":
		default:
			return &domain.ValidationError{Message: "강조색을 목록에서 선택해주세요."}
		}
		result := tx.Model(&SettingsModel{}).Where("id = 1 AND revision = ?", revision).Updates(map[string]any{
			"title": row.Title, "subtitle": row.Subtitle, "theme": row.Theme, "accent": row.Accent, "compact": row.Compact, "custom_css": row.CustomCSS, "revision": revision + 1,
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
	return domain.Settings{Title: row.Title, Subtitle: row.Subtitle, Theme: row.Theme, Accent: row.Accent, Compact: row.Compact, CustomCSS: row.CustomCSS, Revision: row.Revision}
}
