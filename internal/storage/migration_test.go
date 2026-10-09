package storage

import (
	"path/filepath"
	"strings"
	"testing"

	"homedashboard/internal/domain"
)

func TestUpgradePreservesExistingLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	store := &Repository{DB: db}
	if _, err := store.Setup(t.Context(), "owner", "already-hashed"); err != nil {
		t.Fatal(err)
	}
	boards, err := store.Dashboards(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	board, err := store.Dashboard(t.Context(), boards[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	link, err := store.CreateService(t.Context(), domain.ServiceInput{GroupID: board.Groups[0].ID, Name: "기존 링크", URL: "http://192.168.1.2:8080", IconKey: "photos", Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	// Recreate the v0.2 services columns while retaining its actual relationships.
	for _, column := range []string{"icon_mode", "icon_url", "icon_asset_id", "web_check_enabled"} {
		if err := db.Exec("ALTER TABLE services DROP COLUMN " + column).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("DELETE FROM migrations WHERE id = ?", "20261003_link_icons_monitoring").Error; err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	_ = pool.Close()
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	pool, _ = db.DB()
	defer pool.Close()
	upgraded, err := (&Repository{DB: db}).Service(t.Context(), link.ID)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.URL != link.URL || upgraded.Name != link.Name || upgraded.IconKey != "photos" || upgraded.IconMode != "builtin" || !upgraded.Hidden || !upgraded.WebCheckEnabled || upgraded.Revision != link.Revision {
		t.Fatalf("upgrade changed link: %+v", upgraded)
	}
}

func TestWebDashUpgradePreservesUserCustomizationAndRemovesDefaultCopy(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "defaults", true: "custom"}[custom], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "upgrade.db")
			db, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			store := &Repository{DB: db}
			if _, err := store.Setup(t.Context(), "owner", "hash"); err != nil {
				t.Fatal(err)
			}
			title, subtitle := "내 홈서버", "자주 쓰는 링크, 한곳에서."
			if custom {
				title, subtitle = "내 연구실", "직접 작성한 설명"
			}
			if err := db.Exec("UPDATE settings SET title = ?, subtitle = ?, theme = 'dark', compact = 1, custom_css = '.service-card { margin: 0; }', revision = 7", title, subtitle).Error; err != nil {
				t.Fatal(err)
			}
			for _, column := range []string{"appearance_json", "widgets_json"} {
				if err := db.Exec("ALTER TABLE settings DROP COLUMN " + column).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Exec("DELETE FROM migrations WHERE id = ?", "20261009_web_dash_customization").Error; err != nil {
				t.Fatal(err)
			}
			pool, _ := db.DB()
			_ = pool.Close()
			db, err = Open(path)
			if err != nil {
				t.Fatal(err)
			}
			pool, _ = db.DB()
			defer pool.Close()
			settings, err := (&Repository{DB: db}).Settings(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !custom {
				title, subtitle = "web-dash", ""
			}
			if settings.Title != title || settings.Subtitle != subtitle || settings.Theme != "dark" || !settings.Compact || !strings.Contains(settings.CustomCSS, "margin") || settings.Revision != 8 || !settings.Appearance.ShowClock || settings.Widgets.RefreshSeconds != 30 {
				t.Fatalf("migration lost settings: %+v", settings)
			}
		})
	}
}
