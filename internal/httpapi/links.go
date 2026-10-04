package httpapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"homedashboard/internal/domain"
	"homedashboard/internal/icons"
)

func (a *API) linkRoutes(private *gin.RouterGroup) {
	private.POST("/icons", func(c *gin.Context) {
		if err := c.Request.ParseMultipartForm(2 << 20); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				failure(c, 413, "PAYLOAD_TOO_LARGE", "아이콘 파일은 1MB까지 업로드할 수 있습니다.")
			} else {
				failure(c, 400, "VALIDATION_ERROR", "아이콘 파일을 선택해주세요.")
			}
			return
		}
		defer c.Request.MultipartForm.RemoveAll()
		file, _, err := c.Request.FormFile("file")
		if err != nil {
			failure(c, 400, "VALIDATION_ERROR", "아이콘 파일을 선택해주세요.")
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, icons.MaxUpload+1))
		if err != nil {
			respondError(c, err)
			return
		}
		if len(data) > icons.MaxUpload {
			failure(c, 413, "PAYLOAD_TOO_LARGE", "아이콘 파일은 1MB까지 업로드할 수 있습니다.")
			return
		}
		id, err := a.Icons.Save(data)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 201, gin.H{"id": id})
	})
	private.GET("/icons/:id", func(c *gin.Context) {
		img, err := a.Icons.Uploaded(c.Param("id"))
		if err != nil {
			respondError(c, err)
			return
		}
		c.Data(200, img.MIME, img.Data)
	})
	private.GET("/services/:id/icon", func(c *gin.Context) {
		link, err := a.Store.Service(c.Request.Context(), c.Param("id"))
		if err != nil {
			respondError(c, err)
			return
		}
		img := a.Icons.ForLink(c.Request.Context(), link)
		if len(img.Data) == 0 {
			c.Status(204)
			return
		}
		c.Data(200, img.MIME, img.Data)
	})
	statuses := func(refresh bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			dashboard, err := a.Store.Dashboard(c.Request.Context(), c.Param("id"))
			if err != nil {
				respondError(c, err)
				return
			}
			links := []domain.Service{}
			for _, group := range dashboard.Groups {
				links = append(links, group.Services...)
			}
			success(c, 200, a.Monitor.Snapshot(links, refresh))
		}
	}
	private.GET("/dashboards/:id/web-statuses", statuses(false))
	private.POST("/dashboards/:id/web-statuses/check", statuses(true))
}
