package storage

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"homedashboard/internal/domain"
)

func (r *Repository) Addons(ctx context.Context) ([]AddonModel, error) {
	rows := []AddonModel{}
	err := r.DB.WithContext(ctx).Order("position, id").Find(&rows).Error
	return rows, err
}
func (r *Repository) Addon(ctx context.Context, id string) (AddonModel, error) {
	var row AddonModel
	err := r.DB.WithContext(ctx).First(&row, "id = ?", id).Error
	return row, recordError(err)
}
func (r *Repository) SaveAddon(ctx context.Context, row AddonModel, revision int64) (AddonModel, error) {
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if row.ID == "" {
			var count int64
			if err := tx.Model(&AddonModel{}).Count(&count).Error; err != nil {
				return err
			}
			if count >= 24 {
				return &domain.ValidationError{Message: "애드온은 최대 24개까지 설치할 수 있습니다."}
			}
			var position int
			if err := tx.Model(&AddonModel{}).Select("COALESCE(MAX(position), -1) + 1").Scan(&position).Error; err != nil {
				return err
			}
			row.ID, row.Position, row.Revision = uuid.NewString(), position, 1
			return tx.Create(&row).Error
		}
		var old AddonModel
		if err := tx.First(&old, "id = ?", row.ID).Error; err != nil {
			return recordError(err)
		}
		if old.Revision != revision {
			return domain.ErrConflict
		}
		row.Position, row.Revision = old.Position, revision+1
		return tx.Model(&AddonModel{}).Where("id = ? AND revision = ?", row.ID, revision).Updates(map[string]any{"manifest_json": row.ManifestJSON, "values_json": row.ValuesJSON, "secrets_cipher": row.SecretsCipher, "enabled": row.Enabled, "revision": row.Revision}).Error
	})
	return row, err
}
func (r *Repository) DeleteAddon(ctx context.Context, id string, revision int64) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row AddonModel
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			return recordError(err)
		}
		if row.Revision != revision {
			return domain.ErrConflict
		}
		return tx.Delete(&row).Error
	})
}
func (r *Repository) MoveAddon(ctx context.Context, id string, revision int64, direction int) error {
	if direction != -1 && direction != 1 {
		return &domain.ValidationError{Message: "이동 방향을 확인해주세요."}
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []AddonModel
		if err := tx.Order("position, id").Find(&rows).Error; err != nil {
			return err
		}
		index := -1
		for i, row := range rows {
			if row.ID == id {
				index = i
				break
			}
		}
		if index < 0 {
			return domain.ErrNotFound
		}
		if rows[index].Revision != revision {
			return domain.ErrConflict
		}
		target := index + direction
		if target < 0 || target >= len(rows) {
			return nil
		}
		rows[index], rows[target] = rows[target], rows[index]
		for i, row := range rows {
			if err := tx.Model(&AddonModel{}).Where("id = ?", row.ID).Updates(map[string]any{"position": i, "revision": row.Revision + 1}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
