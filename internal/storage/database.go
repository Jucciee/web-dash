package storage

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"homedashboard/internal/domain"
)

func Open(path string) (*gorm.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uriPath := filepath.ToSlash(absolute)
	// SQLite's time format converts every bound timestamp to UTC, including
	// gormstore's time.Now() queries, so session expiry comparisons are ordered.
	q := url.Values{"_pragma": {"busy_timeout(5000)", "foreign_keys(1)", "journal_mode(WAL)"}, "_txlock": {"immediate"}, "_timefmt": {"sqlite"}}
	// SQLite's Go VFS expects Windows drive paths as file:C:/... (no leading slash).
	uri := "file:" + (&url.URL{Path: uriPath}).EscapedPath() + "?" + q.Encode()
	db, err := gorm.Open(gormlite.Open(uri), &gorm.Config{
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Silent),
		NowFunc:        func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(1)
	pool.SetMaxIdleConns(1)
	if err := gormigrate.New(db, &gormigrate.Options{TableName: "migrations", IDColumnName: "id", IDColumnSize: 255, UseTransaction: true, ValidateUnknownMigrations: true}, []*gormigrate.Migration{
		{ID: "20261003_initial", Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&UserModel{}, &DashboardModel{}, &GroupModel{}, &ServiceModel{}, &SessionModel{})
		}},
		{ID: "20261003_settings_imports", Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&SettingsModel{}, &ImportModel{}); err != nil {
				return err
			}
			return tx.Create(&SettingsModel{ID: 1, Title: "내 홈서버", Subtitle: "자주 쓰는 서비스, 한곳에서.", Theme: "light", Accent: "blue", Revision: 1}).Error
		}},
		{ID: "20261003_link_icons_monitoring", Migrate: func(tx *gorm.DB) error {
			// Existing links start with monitoring enabled. Keep the model without a
			// GORM default so explicit false values also survive INSERTs.
			if !tx.Migrator().HasColumn(&ServiceModel{}, "web_check_enabled") {
				if err := tx.Exec("ALTER TABLE services ADD COLUMN web_check_enabled numeric NOT NULL DEFAULT 1").Error; err != nil {
					return err
				}
			}
			if err := tx.AutoMigrate(&ServiceModel{}); err != nil {
				return err
			}
			if err := tx.Exec("UPDATE dashboards SET revision = revision + 1 WHERE id IN (SELECT dashboard_id FROM groups WHERE name = ? AND position = 0 AND revision = 1)", "서비스").Error; err != nil {
				return err
			}
			if err := tx.Model(&GroupModel{}).Where("name = ? AND position = 0 AND revision = 1", "서비스").Updates(map[string]any{"name": "링크", "revision": gorm.Expr("revision + 1")}).Error; err != nil {
				return err
			}
			return tx.Model(&SettingsModel{}).Where("id = 1 AND subtitle = ?", "자주 쓰는 서비스, 한곳에서.").Updates(map[string]any{"subtitle": "자주 쓰는 링크, 한곳에서.", "revision": gorm.Expr("revision + CASE WHEN EXISTS (SELECT 1 FROM users) THEN 1 ELSE 0 END")}).Error
		}},
		{ID: "20261009_web_dash_customization", Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&SettingsModel{}); err != nil {
				return err
			}
			appearance, _ := json.Marshal(domain.DefaultAppearance())
			widgets, _ := json.Marshal(domain.DefaultWidgets())
			if err := tx.Exec(`UPDATE settings SET
				title = CASE WHEN title IN ('내 홈서버', '나의 홈서버') THEN 'web-dash' ELSE title END,
				subtitle = CASE WHEN subtitle IN ('자주 쓰는 서비스, 한곳에서.', '자주 쓰는 링크, 한곳에서.', '매일 쓰는 링크, 한곳에서 열어보세요.') THEN '' ELSE subtitle END,
				appearance_json = ?, widgets_json = ?,
				revision = revision + CASE WHEN EXISTS (SELECT 1 FROM users) THEN 1 ELSE 0 END`, string(appearance), string(widgets)).Error; err != nil {
				return err
			}
			return tx.Model(&DashboardModel{}).Where("name IN ?", []string{"내 홈서버", "나의 홈서버"}).Updates(map[string]any{"name": "web-dash", "revision": gorm.Expr("revision + 1")}).Error
		}},
		{ID: "20261010_api_addons", Migrate: func(tx *gorm.DB) error { return tx.AutoMigrate(&AddonModel{}) }},
	}).Migrate(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}
	return db, nil
}
