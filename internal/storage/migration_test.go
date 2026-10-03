package storage

import (
	"path/filepath"
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
