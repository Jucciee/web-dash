package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"homedashboard/internal/addons"
	"homedashboard/internal/auth"
	"homedashboard/internal/backgrounds"
	"homedashboard/internal/domain"
	"homedashboard/internal/icons"
	"homedashboard/internal/imports"
	"homedashboard/internal/monitoring"
	"homedashboard/internal/services"
	"homedashboard/internal/static"
	"homedashboard/internal/storage"
	"homedashboard/internal/telemetry"
)

type API struct {
	Auth        *auth.Service
	Dashboard   *services.Dashboard
	Store       *storage.Repository
	OnSetup     func()
	Imports     *imports.Service
	Icons       *icons.Store
	Monitor     *monitoring.Monitor
	Telemetry   *telemetry.Collector
	Backgrounds *backgrounds.Store
	Addons      *addons.Service
}

func (a *API) Router(staticDir string) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.Use(func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("request panic recovered", "requestId", requestID(c.Request.Context()), "method", c.Request.Method, "path", c.Request.URL.Path)
				failure(c, 500, "INTERNAL_ERROR", "요청을 처리하지 못했습니다.")
			}
		}()
		c.Next()
	})
	api := router.Group("/api/v1")
	api.GET("/health/live", func(c *gin.Context) { success(c, 200, gin.H{"status": "ok"}) })
	api.GET("/setup/status", func(c *gin.Context) {
		needsSetup, err := a.Store.NeedsSetup(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, gin.H{"needsSetup": needsSetup})
	})
	api.GET("/auth/csrf", func(c *gin.Context) { success(c, 200, gin.H{"csrfToken": csrf.Token(c.Request)}) })
	limit := authLimiter()
	api.POST("/setup", limit, a.setup)
	api.POST("/sessions", limit, a.login)
	private := api.Group("")
	private.Use(func(c *gin.Context) {
		user, err := a.Auth.CurrentUser(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		c.Set("user", user)
		c.Next()
	})
	a.settingsRoutes(private)
	a.telemetryRoutes(private)
	a.addonRoutes(private)
	a.backgroundRoutes(private)
	a.importRoutes(private)
	a.linkRoutes(private)
	private.GET("/me", func(c *gin.Context) {
		success(c, 200, gin.H{"user": c.MustGet("user"), "csrfToken": csrf.Token(c.Request)})
	})
	private.DELETE("/sessions/current", func(c *gin.Context) {
		if err := a.Auth.Sessions.Destroy(c.Request.Context()); err != nil {
			respondError(c, err)
			return
		}
		c.Status(204)
	})
	private.GET("/dashboards", func(c *gin.Context) {
		rows, err := a.Store.Dashboards(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, rows)
	})
	private.GET("/dashboards/:id", func(c *gin.Context) {
		row, err := a.Store.Dashboard(c.Request.Context(), c.Param("id"))
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, row)
	})
	private.GET("/services", func(c *gin.Context) {
		if _, err := uuid.Parse(c.Query("dashboardId")); err != nil {
			failure(c, 400, "VALIDATION_ERROR", "dashboardId가 필요합니다.")
			return
		}
		row, err := a.Store.Dashboard(c.Request.Context(), c.Query("dashboardId"))
		if err != nil {
			respondError(c, err)
			return
		}
		items := []domain.Service{}
		for _, group := range row.Groups {
			items = append(items, group.Services...)
		}
		success(c, 200, items)
	})
	private.POST("/services", func(c *gin.Context) {
		var input domain.ServiceInput
		if !bind(c, &input) {
			return
		}
		row, err := a.Dashboard.Create(c.Request.Context(), input)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 201, row)
	})
	private.PATCH("/services/:id", func(c *gin.Context) {
		revision, ok := expectedRevision(c)
		if !ok {
			return
		}
		var patch domain.ServicePatch
		if !bind(c, &patch) {
			return
		}
		row, err := a.Dashboard.Update(c.Request.Context(), c.Param("id"), revision, patch)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, row)
	})
	private.DELETE("/services/:id", func(c *gin.Context) {
		revision, ok := expectedRevision(c)
		if !ok {
			return
		}
		if err := a.Store.DeleteService(c.Request.Context(), c.Param("id"), revision); err != nil {
			respondError(c, err)
			return
		}
		c.Status(204)
	})
	private.POST("/dashboards/:id/groups", func(c *gin.Context) {
		name, ok := groupName(c)
		if !ok {
			return
		}
		row, err := a.Store.CreateGroup(c.Request.Context(), c.Param("id"), name)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 201, row)
	})
	private.PATCH("/groups/:id", func(c *gin.Context) {
		revision, ok := expectedRevision(c)
		if !ok {
			return
		}
		name, ok := groupName(c)
		if !ok {
			return
		}
		row, err := a.Store.UpdateGroup(c.Request.Context(), c.Param("id"), name, revision)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, row)
	})
	private.DELETE("/groups/:id", func(c *gin.Context) {
		revision, ok := expectedRevision(c)
		if !ok {
			return
		}
		if err := a.Store.DeleteGroup(c.Request.Context(), c.Param("id"), revision); err != nil {
			respondError(c, err)
			return
		}
		c.Status(204)
	})
	private.PUT("/dashboards/:id/layout", func(c *gin.Context) {
		revision, ok := expectedRevision(c)
		if !ok {
			return
		}
		var input domain.Layout
		if !bind(c, &input) {
			return
		}
		if err := a.Store.SaveLayout(c.Request.Context(), c.Param("id"), revision, input); err != nil {
			respondError(c, err)
			return
		}
		row, err := a.Store.Dashboard(c.Request.Context(), c.Param("id"))
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, row)
	})
	spa := static.Handler(staticDir)
	router.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == "/api" || strings.HasPrefix(c.Request.URL.Path, "/api/") {
			failure(c, 404, "NOT_FOUND", "요청한 API를 찾을 수 없습니다.")
			return
		}
		spa.ServeHTTP(c.Writer, c.Request)
	})
	return router
}

func (a *API) setup(c *gin.Context) {
	var input struct {
		SetupToken string `json:"setupToken" binding:"required,max=256"`
		Username   string `json:"username" binding:"required,max=64"`
		Password   string `json:"password" binding:"required"`
	}
	if !bind(c, &input) {
		return
	}
	user, err := a.Auth.Setup(c.Request.Context(), input.SetupToken, input.Username, input.Password)
	if err != nil {
		respondError(c, err)
		return
	}
	if a.OnSetup != nil {
		a.OnSetup()
	}
	if err := a.Auth.StartSession(c.Request.Context(), user); err != nil {
		respondError(c, err)
		return
	}
	success(c, 201, gin.H{"user": user, "csrfToken": csrf.Token(c.Request)})
}

func (a *API) login(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required,max=64"`
		Password string `json:"password" binding:"required"`
	}
	if !bind(c, &input) {
		return
	}
	user, err := a.Auth.Login(c.Request.Context(), input.Username, input.Password)
	if err != nil {
		respondError(c, err)
		return
	}
	if err := a.Auth.StartSession(c.Request.Context(), user); err != nil {
		respondError(c, err)
		return
	}
	success(c, 200, gin.H{"user": user, "csrfToken": csrf.Token(c.Request)})
}

func bind(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			failure(c, 413, "PAYLOAD_TOO_LARGE", "요청 크기가 제한을 초과했습니다.")
		} else {
			failure(c, 400, "VALIDATION_ERROR", "입력 형식과 필수 항목을 확인해주세요.")
		}
		return false
	}
	return true
}

func expectedRevision(c *gin.Context) (int64, bool) {
	header := c.GetHeader("If-Match")
	if len(header) < 3 || header[0] != '"' || header[len(header)-1] != '"' {
		failure(c, 400, "REVISION_REQUIRED", "If-Match에 따옴표로 감싼 revision을 보내주세요.")
		return 0, false
	}
	revision, err := strconv.ParseInt(header[1:len(header)-1], 10, 64)
	if err != nil || revision < 1 {
		failure(c, 400, "REVISION_REQUIRED", "유효한 revision이 필요합니다.")
		return 0, false
	}
	return revision, true
}

func groupName(c *gin.Context) (string, bool) {
	var input struct {
		Name string `json:"name" binding:"required,max=80"`
	}
	if !bind(c, &input) {
		return "", false
	}
	name, err := services.ValidateName(input.Name)
	if err != nil {
		respondError(c, err)
		return "", false
	}
	return name, true
}

func authLimiter() gin.HandlerFunc {
	type entry struct {
		start time.Time
		count int
	}
	var mu sync.Mutex
	entries := map[string]entry{}
	return func(c *gin.Context) {
		now := time.Now()
		key := c.ClientIP()
		mu.Lock()
		for ip, item := range entries {
			if now.Sub(item.start) >= time.Minute {
				delete(entries, ip)
			}
		}
		item := entries[key]
		if item.start.IsZero() && len(entries) >= 1024 {
			mu.Unlock()
			c.Header("Retry-After", "60")
			failure(c, 429, "RATE_LIMITED", "로그인 요청이 많습니다. 1분 후 다시 시도해주세요.")
			return
		}
		if item.start.IsZero() {
			item.start = now
		}
		item.count++
		allowed := item.count <= 10
		entries[key] = item
		mu.Unlock()
		if !allowed {
			c.Header("Retry-After", "60")
			failure(c, 429, "RATE_LIMITED", "로그인 요청이 많습니다. 1분 후 다시 시도해주세요.")
			return
		}
		c.Next()
	}
}

func Wrap(next http.Handler, sessions func(http.Handler) http.Handler, key []byte, secure bool) http.Handler {
	csrfHandler := csrf.Protect(key, csrf.Secure(secure), csrf.Path("/"), csrf.CookieName("dashboard_csrf"), csrf.SameSite(csrf.SameSiteStrictMode), csrf.RequestHeader("X-CSRF-Token"), csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, 403, "CSRF_INVALID", "보안 토큰을 확인하지 못했습니다. 페이지를 새로고침해주세요.")
	})))(next)
	inner := sessions(csrfHandler)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id))
		if !secure {
			r = csrf.PlaintextHTTPRequest(r)
		}
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		bodyLimit := int64(64 << 10)
		if r.Method == http.MethodPatch && r.URL.Path == "/api/v1/settings" {
			bodyLimit = 256 << 10
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/imports/preview" {
			bodyLimit = 12 << 20
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/icons" {
			bodyLimit = icons.MaxUpload + (64 << 10)
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/backgrounds" {
			bodyLimit = backgrounds.MaxUpload + (64 << 10)
		}
		r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)
		metrics := httpsnoop.CaptureMetrics(inner, w, r)
		level := slog.LevelInfo
		if metrics.Code >= 500 {
			level = slog.LevelError
		} else if metrics.Code >= 400 {
			level = slog.LevelWarn
		}
		if r.URL.Path == "/api/v1/health/live" || !strings.HasPrefix(r.URL.Path, "/api/") {
			level = slog.LevelDebug
		}
		slog.Log(r.Context(), level, "http request", "requestId", id, "method", r.Method, "path", r.URL.Path, "status", metrics.Code, "durationMs", metrics.Duration.Milliseconds(), "bytes", metrics.Written)
	})
}
