package httpapi

import (
	"github.com/gin-gonic/gin"
	"homedashboard/internal/domain"
)

func (a *API) importRoutes(private *gin.RouterGroup) {
	private.POST("/imports/preview", func(c *gin.Context) {
		var input domain.ImportInput
		if !bind(c, &input) {
			return
		}
		result, err := a.Imports.Preview(c.Request.Context(), c.MustGet("user").(domain.User).ID, input)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 201, result)
	})
	private.POST("/imports/docker/scan", func(c *gin.Context) {
		var input domain.DockerScanInput
		if !bind(c, &input) {
			return
		}
		result, err := a.Imports.ScanDocker(c.Request.Context(), c.MustGet("user").(domain.User).ID, input)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 201, result)
	})
	private.GET("/imports/:id", func(c *gin.Context) {
		result, err := a.Store.Preview(c.Request.Context(), c.MustGet("user").(domain.User).ID, c.Param("id"))
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, result)
	})
	private.POST("/imports/:id/commit", func(c *gin.Context) {
		var input domain.ImportCommit
		if !bind(c, &input) {
			return
		}
		result, err := a.Imports.Commit(c.Request.Context(), c.MustGet("user").(domain.User).ID, c.Param("id"), c.GetHeader("Idempotency-Key"), input)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, result)
	})
}
