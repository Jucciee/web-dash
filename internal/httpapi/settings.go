package httpapi

import (
	"github.com/gin-gonic/gin"
	"homedashboard/internal/domain"
)

func (a *API) settingsRoutes(private *gin.RouterGroup) {
	private.GET("/settings", func(c *gin.Context) {
		settings, err := a.Store.Settings(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, settings)
	})
	private.PATCH("/settings", func(c *gin.Context) {
		revision, ok := expectedRevision(c)
		if !ok {
			return
		}
		var patch domain.SettingsPatch
		if !bind(c, &patch) {
			return
		}
		settings, err := a.Store.UpdateSettings(c.Request.Context(), revision, patch)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, settings)
	})
	private.GET("/settings/custom.css", func(c *gin.Context) {
		settings, err := a.Store.Settings(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		c.Data(200, "text/css; charset=utf-8", []byte(settings.CustomCSS))
	})
}
