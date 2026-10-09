package httpapi

import "github.com/gin-gonic/gin"

func (a *API) telemetryRoutes(private *gin.RouterGroup) {
	private.GET("/monitoring/host", func(c *gin.Context) {
		result, err := a.Telemetry.Host(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, result)
	})
	private.GET("/monitoring/docker", func(c *gin.Context) {
		result, err := a.Telemetry.Docker(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, result)
	})
}
