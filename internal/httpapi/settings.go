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
		if patch.Appearance != nil && patch.Appearance.BackgroundImage != "" {
			if _, _, err := a.Backgrounds.Read(patch.Appearance.BackgroundImage); err != nil {
				respondError(c, &domain.ValidationError{Message: "배경 이미지가 없습니다. 이미지를 다시 업로드해주세요."})
				return
			}
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
