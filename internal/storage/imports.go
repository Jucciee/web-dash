package storage

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"homedashboard/internal/domain"
)

type ImportModel struct {
	ID                                string `gorm:"primaryKey;size:36"`
	UserID                            string `gorm:"not null;index"`
	DashboardID                       string `gorm:"not null;index"`
	Kind, SourceName                  string
	DashboardRevision                 int64
	PreviewJSON                       string    `gorm:"not null"`
	ExpiresAt                         time.Time `gorm:"not null;index"`
	CommitKey, CommitHash, ResultJSON string
}

func (ImportModel) TableName() string { return "imports" }

func CanonicalURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Host = strings.ToLower(u.Host)
	if u.Path == "/" {
		u.Path = ""
	}
	return u.String()
}

func (r *Repository) SavePreview(ctx context.Context, userID string, preview domain.ImportPreview) error {
	data, err := json.Marshal(preview)
	if err != nil {
		return err
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at < ?", time.Now().UTC()).Delete(&ImportModel{}).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&ImportModel{}).Count(&count).Error; err != nil {
			return err
		}
		if count >= 100 {
			return &domain.ImportError{Code: "IMPORT_LIMIT", Message: "미리보기가 많습니다. 기존 미리보기 만료 후 다시 시도해주세요."}
		}
		return tx.Create(&ImportModel{ID: preview.ID, UserID: userID, DashboardID: preview.DashboardID, Kind: preview.Kind, SourceName: preview.SourceName, DashboardRevision: preview.DashboardRevision, PreviewJSON: string(data), ExpiresAt: preview.ExpiresAt}).Error
	})
}

func (r *Repository) Preview(ctx context.Context, userID, id string) (domain.ImportPreview, error) {
	var row ImportModel
	if err := r.DB.WithContext(ctx).First(&row, "id = ? AND user_id = ?", id, userID).Error; err != nil {
		return domain.ImportPreview{}, recordError(err)
	}
	if time.Now().After(row.ExpiresAt) {
		return domain.ImportPreview{}, &domain.ImportError{Code: "IMPORT_EXPIRED", Message: "미리보기가 만료되었습니다. 다시 분석해주세요."}
	}
	var result domain.ImportPreview
	err := json.Unmarshal([]byte(row.PreviewJSON), &result)
	return result, err
}

func (r *Repository) CommitImport(ctx context.Context, userID, id, key, hash string, input domain.ImportCommit) (domain.ImportResult, error) {
	result := domain.ImportResult{ServiceIDs: []string{}}
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row ImportModel
		if err := tx.First(&row, "id = ? AND user_id = ?", id, userID).Error; err != nil {
			return recordError(err)
		}
		if time.Now().After(row.ExpiresAt) {
			return &domain.ImportError{Code: "IMPORT_EXPIRED", Message: "미리보기가 만료되었습니다. 다시 분석해주세요."}
		}
		if row.CommitKey != "" {
			if row.CommitKey != key || row.CommitHash != hash {
				return &domain.ImportError{Code: "IMPORT_COMMITTED", Message: "이미 적용한 미리보기입니다. 다시 분석해주세요."}
			}
			return json.Unmarshal([]byte(row.ResultJSON), &result)
		}
		var dashboard DashboardModel
		if err := tx.First(&dashboard, "id = ?", row.DashboardID).Error; err != nil {
			return recordError(err)
		}
		if dashboard.Revision != row.DashboardRevision || dashboard.Revision != input.DashboardRevision {
			return domain.ErrConflict
		}
		var preview domain.ImportPreview
		if err := json.Unmarshal([]byte(row.PreviewJSON), &preview); err != nil {
			return err
		}
		allowed := map[string]bool{}
		for _, candidate := range preview.Candidates {
			allowed[candidate.ID] = true
		}
		var existing []ServiceModel
		if err := tx.Joins("JOIN groups ON groups.id = services.group_id").Where("groups.dashboard_id = ?", dashboard.ID).Find(&existing).Error; err != nil {
			return err
		}
		urls := map[string]bool{}
		for _, service := range existing {
			urls[CanonicalURL(service.URL)] = true
		}
		for _, item := range input.Items {
			if !allowed[item.CandidateID] {
				return &domain.ValidationError{Message: "미리보기 후보를 한 번씩 선택해주세요."}
			}
			delete(allowed, item.CandidateID)
			if urls[CanonicalURL(item.URL)] {
				return &domain.ImportError{Code: "IMPORT_DUPLICATE", Message: "이미 등록된 주소가 포함되어 있습니다. 중복 항목을 제외하고 다시 분석해주세요."}
			}
			urls[CanonicalURL(item.URL)] = true
			var group GroupModel
			if err := tx.First(&group, "id = ? AND dashboard_id = ?", item.GroupID, dashboard.ID).Error; err != nil {
				return &domain.ValidationError{Message: "이 대시보드의 그룹을 선택해주세요."}
			}
			position, err := nextPosition(tx, &ServiceModel{}, "group_id = ?", group.ID)
			if err != nil {
				return err
			}
			service := serviceModel(item.ServiceInput)
			service.ID, service.Position, service.Revision = uuid.NewString(), position, 1
			if err := tx.Create(&service).Error; err != nil {
				return err
			}
			result.ServiceIDs = append(result.ServiceIDs, service.ID)
		}
		if err := bumpDashboard(tx, dashboard.ID); err != nil {
			return err
		}
		result.DashboardRevision = dashboard.Revision + 1
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return tx.Model(&row).Updates(map[string]any{"commit_key": key, "commit_hash": hash, "result_json": string(data)}).Error
	})
	return result, err
}
