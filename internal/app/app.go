package app

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/alexedwards/scs/gormstore"
	"github.com/alexedwards/scs/v2"
	"homedashboard/internal/addons"
	"homedashboard/internal/auth"
	"homedashboard/internal/backgrounds"
	"homedashboard/internal/domain"
	"homedashboard/internal/httpapi"
	"homedashboard/internal/icons"
	"homedashboard/internal/imports"
	"homedashboard/internal/monitoring"
	"homedashboard/internal/services"
	"homedashboard/internal/storage"
	"homedashboard/internal/telemetry"
	"homedashboard/internal/webfetch"
)

type App struct {
	Handler http.Handler
	close   func() error
}

func New(cfg Config) (*App, error) {
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return nil, err
	}
	db, err := storage.Open(filepath.Join(cfg.DataDir, "dashboard.db"))
	if err != nil {
		return nil, err
	}
	pool, _ := db.DB()
	completed := false
	defer func() {
		if !completed {
			pool.Close()
		}
	}()
	store := &storage.Repository{DB: db}
	csrfKey, err := persistentSecret(filepath.Join(cfg.DataDir, "csrf.key"))
	if err != nil {
		return nil, err
	}
	addonKey, err := persistentSecret(filepath.Join(cfg.DataDir, "addon-secret.key"))
	if err != nil {
		return nil, err
	}
	addonService, err := addons.New(store, addonKey)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !completed {
			addonService.Close()
		}
	}()
	needsSetup, err := store.NeedsSetup(context.Background())
	if err != nil {
		return nil, err
	}
	tokenPath := filepath.Join(cfg.DataDir, "setup-token")
	if needsSetup && cfg.SetupToken == "" {
		key, err := persistentSecret(tokenPath)
		if err != nil {
			return nil, err
		}
		cfg.SetupToken = base64.RawURLEncoding.EncodeToString(key)
		slog.Info("initial setup required; read the token file", "path", tokenPath)
	}
	if !needsSetup {
		_ = os.Remove(tokenPath)
		cfg.SetupToken = ""
	}
	sessionStore, err := gormstore.New(db)
	if err != nil {
		return nil, err
	}
	sessions := scs.New()
	sessions.Store = sessionStore
	sessions.Lifetime = 24 * time.Hour
	sessions.IdleTimeout = 2 * time.Hour
	sessions.Cookie.Name = "dashboard_session"
	sessions.Cookie.HttpOnly = true
	sessions.Cookie.Secure = cfg.CookieSecure
	sessions.Cookie.SameSite = http.SameSiteStrictMode
	sessions.Cookie.Persist = true
	sessions.ErrorFunc = func(w http.ResponseWriter, _ *http.Request, err error) {
		slog.Error("session failed", "error", err)
		http.Error(w, "Session unavailable", 500)
	}
	authService, err := auth.New(store, sessions, cfg.SetupToken)
	if err != nil {
		sessionStore.StopCleanup()
		return nil, err
	}
	webClient := webfetch.NewClient()
	iconStore, err := icons.New(filepath.Join(cfg.DataDir, "icons"), webClient)
	if err != nil {
		sessionStore.StopCleanup()
		webClient.CloseIdleConnections()
		return nil, err
	}
	validateAsset := func(id string) error {
		if _, err := iconStore.Uploaded(id); err != nil {
			return &domain.ValidationError{Message: "업로드된 아이콘 파일을 찾지 못했습니다. 파일을 다시 선택해주세요."}
		}
		return nil
	}
	monitor := monitoring.New(webClient)
	metrics := telemetry.New(telemetry.Config{DockerHost: cfg.DockerHost, HostProc: cfg.HostProc, HostSys: cfg.HostSys, HostRoot: cfg.HostRoot, HostDisk: cfg.HostDisk, HostName: cfg.HostName})
	backgroundStore, err := backgrounds.New(filepath.Join(cfg.DataDir, "backgrounds"))
	if err != nil {
		metrics.Close()
		monitor.Close()
		sessionStore.StopCleanup()
		webClient.CloseIdleConnections()
		return nil, err
	}
	api := &httpapi.API{Auth: authService, Store: store, Icons: iconStore, Monitor: monitor, Dashboard: &services.Dashboard{Store: store, ValidateIconAsset: validateAsset}, Imports: &imports.Service{Store: store, Docker: imports.EngineReader{Host: cfg.DockerHost}, ValidateIconAsset: validateAsset}, OnSetup: func() {
		if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("could not remove consumed setup token file", "error", err)
		}
	}}
	api.Telemetry, api.Backgrounds = metrics, backgroundStore
	api.Addons = addonService
	completed = true
	return &App{Handler: httpapi.Wrap(api.Router(cfg.StaticDir), sessions.LoadAndSave, csrfKey, cfg.CookieSecure), close: func() error {
		addonService.Close()
		monitor.Close()
		metrics.Close()
		webClient.CloseIdleConnections()
		sessionStore.StopCleanup()
		return pool.Close()
	}}, nil
}

func (a *App) Close() error { return a.close() }
