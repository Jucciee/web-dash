package storage

import (
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"testing"

	"homedashboard/internal/domain"
)

func homeCoordinate(value int) *int { return &value }

func exampleHomeLayout() domain.HomeLayout {
	return domain.HomeLayout{Version: 1, Mode: "custom", Sections: []domain.HomeSection{{
		ID: "a6e0bf3d-8f29-46ee-aafb-2fa1a65d37b1", Name: "사진과 서버", X: homeCoordinate(6), Y: homeCoordinate(4), Width: 6, MinRows: 0, Collapsed: false,
		Items: []domain.HomeItem{
			{Kind: "host", X: 0, Y: 0, Width: 6, Height: 7},
			{Kind: "addon", RefID: "3aeb71a6-ef1a-4f64-b802-717913897ed5", X: 6, Y: 0, Width: 6, Height: 6},
			{Kind: "link", RefID: "abff03f0-0f45-4529-a673-14b8043b6ce5", X: 0, Y: 7, Width: 4, Height: 3},
		},
	}}}
}

func TestHomeLayoutPersistsAndUsesSettingsRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	store := &Repository{DB: db}
	settings, err := store.Settings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(settings.HomeLayout, domain.DefaultHomeLayout()) {
		t.Fatalf("new/old users should retain auto mode: %+v", settings.HomeLayout)
	}
	layout := exampleHomeLayout()
	saved, err := store.UpdateSettings(t.Context(), settings.Revision, domain.SettingsPatch{HomeLayout: &layout})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.HomeLayout, layout) || saved.Title != settings.Title || saved.Revision != settings.Revision+1 {
		t.Fatalf("layout-only save changed appearance: %+v", saved)
	}
	if _, err := store.UpdateSettings(t.Context(), settings.Revision, domain.SettingsPatch{HomeLayout: &layout}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale save should conflict: %v", err)
	}
	pool, _ := db.DB()
	_ = pool.Close()
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	pool, _ = db.DB()
	defer pool.Close()
	store = &Repository{DB: db}
	reloaded, err := store.Settings(t.Context())
	if err != nil || !reflect.DeepEqual(reloaded.HomeLayout, layout) {
		t.Fatalf("restart lost layout: %+v, %v", reloaded.HomeLayout, err)
	}
	auto := domain.DefaultHomeLayout()
	reset, err := store.UpdateSettings(t.Context(), reloaded.Revision, domain.SettingsPatch{HomeLayout: &auto})
	if err != nil || !reflect.DeepEqual(reset.HomeLayout, auto) {
		t.Fatalf("reset failed: %+v, %v", reset, err)
	}
}

func TestRejectInvalidHomeLayoutWithoutSavingOtherSettings(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "invalid.db"))
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	defer pool.Close()
	store := &Repository{DB: db}
	settings, _ := store.Settings(t.Context())
	layout := exampleHomeLayout()
	layout.Sections[0].Items[1].X = 5 // Overlaps the host card.
	title := "should roll back"
	if _, err := store.UpdateSettings(t.Context(), settings.Revision, domain.SettingsPatch{HomeLayout: &layout, Title: &title}); err == nil {
		t.Fatal("overlapping layout accepted")
	}
	after, _ := store.Settings(t.Context())
	if !reflect.DeepEqual(settings, after) {
		t.Fatalf("invalid save partially committed: %+v", after)
	}
}

func TestHomeLayoutValidation(t *testing.T) {
	cases := map[string]func(*domain.HomeLayout){
		"future version":     func(l *domain.HomeLayout) { l.Version = 2 },
		"auto with sections": func(l *domain.HomeLayout) { l.Mode = "auto" },
		"null sections":      func(l *domain.HomeLayout) { l.Sections = nil },
		"duplicate section":  func(l *domain.HomeLayout) { l.Sections = append(l.Sections, l.Sections[0]) },
		"unknown kind":       func(l *domain.HomeLayout) { l.Sections[0].Items[0].Kind = "script" },
		"duplicate reference": func(l *domain.HomeLayout) {
			l.Sections = append(l.Sections, domain.HomeSection{ID: "606172d4-0274-4645-98fc-be9db2eab33a", Name: "dup", Width: 6, Items: []domain.HomeItem{l.Sections[0].Items[0]}})
		},
		"invalid reference":             func(l *domain.HomeLayout) { l.Sections[0].Items[1].RefID = "not-an-id" },
		"host reference":                func(l *domain.HomeLayout) { l.Sections[0].Items[0].RefID = "secret" },
		"negative coordinate":           func(l *domain.HomeLayout) { l.Sections[0].Items[0].Y = -1 },
		"outside columns":               func(l *domain.HomeLayout) { l.Sections[0].Items[0].Width = 13 },
		"height limit":                  func(l *domain.HomeLayout) { l.Sections[0].Items[0].Height = 25 },
		"row limit":                     func(l *domain.HomeLayout) { l.Sections[0].Items[0].Y = 4095 },
		"coordinate overflow":           func(l *domain.HomeLayout) { l.Sections[0].Items[0].X = math.MaxInt },
		"row overflow":                  func(l *domain.HomeLayout) { l.Sections[0].Items[0].Y = math.MaxInt },
		"empty name":                    func(l *domain.HomeLayout) { l.Sections[0].Name = "  " },
		"section width":                 func(l *domain.HomeLayout) { l.Sections[0].Width = 2 },
		"section height":                func(l *domain.HomeLayout) { l.Sections[0].MinRows = 49 },
		"partial section position":      func(l *domain.HomeLayout) { l.Sections[0].Y = nil },
		"negative section position":     func(l *domain.HomeLayout) { l.Sections[0].X = homeCoordinate(-1) },
		"section outside columns":       func(l *domain.HomeLayout) { l.Sections[0].X = homeCoordinate(7) },
		"section coordinate overflow":   func(l *domain.HomeLayout) { l.Sections[0].X = homeCoordinate(math.MaxInt) },
		"section row overflow":          func(l *domain.HomeLayout) { l.Sections[0].Y = homeCoordinate(math.MaxInt) },
		"section extends beyond canvas": func(l *domain.HomeLayout) { l.Sections[0].Y = homeCoordinate(65535) },
		"overlapping sections": func(l *domain.HomeLayout) {
			l.Sections = append(l.Sections, domain.HomeSection{ID: "606172d4-0274-4645-98fc-be9db2eab33a", Name: "overlap", X: homeCoordinate(6), Y: homeCoordinate(4), Width: 6, Items: []domain.HomeItem{}})
		},
		"null items": func(l *domain.HomeLayout) { l.Sections[0].Items = nil },
	}
	if err := validateHomeLayout(exampleHomeLayout()); err != nil {
		t.Fatal(err)
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			layout := exampleHomeLayout()
			change(&layout)
			if validateHomeLayout(layout) == nil {
				t.Fatal("invalid layout accepted")
			}
		})
	}
}

func TestHomeLayoutSectionPositionsRemainBackwardCompatible(t *testing.T) {
	layout := exampleHomeLayout()
	layout.Sections[0].X, layout.Sections[0].Y = nil, nil
	if err := validateHomeLayout(layout); err != nil {
		t.Fatalf("legacy coordinates should be optional: %v", err)
	}
	layout = exampleHomeLayout()
	layout.Sections = append(layout.Sections, domain.HomeSection{ID: "606172d4-0274-4645-98fc-be9db2eab33a", Name: "beside", X: homeCoordinate(0), Y: homeCoordinate(4), Width: 6, Items: []domain.HomeItem{}})
	if err := validateHomeLayout(layout); err != nil {
		t.Fatalf("adjacent sections should be allowed: %v", err)
	}
}

func TestHomeLayoutMigrationPreservesV04Settings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE settings SET title = ?, theme = 'dark', custom_css = ?, revision = 42", "나의 서버", ".service-card { color: inherit; }").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE settings DROP COLUMN home_layout_json").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM migrations WHERE id = ?", "20261010_home_layout").Error; err != nil {
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
	if err != nil || settings.Title != "나의 서버" || settings.Theme != "dark" || settings.Revision != 42 || settings.CustomCSS != ".service-card { color: inherit; }" || settings.HomeLayout.Mode != "auto" || !settings.Appearance.ShowTitle {
		t.Fatalf("migration changed saved settings: %+v, %v", settings, err)
	}
}
