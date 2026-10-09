package httpapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"homedashboard/internal/backgrounds"
)

func (a *API) backgroundRoutes(private *gin.RouterGroup) {
	private.POST("/backgrounds", func(c *gin.Context) {
		if err := c.Request.ParseMultipartForm(backgrounds.MaxUpload + (64 << 10)); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				failure(c, 413, "PAYLOAD_TOO_LARGE", "배경 이미지는 8MiB까지 업로드할 수 있습니다.")
			} else {
				failure(c, 400, "VALIDATION_ERROR", "배경 이미지를 선택해주세요.")
			}
			return
		}
		defer c.Request.MultipartForm.RemoveAll()
		file, _, err := c.Request.FormFile("file")
		if err != nil {
			failure(c, 400, "VALIDATION_ERROR", "배경 이미지를 선택해주세요.")
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, backgrounds.MaxUpload+1))
		if err != nil {
			respondError(c, err)
			return
		}
		if len(data) > backgrounds.MaxUpload {
			failure(c, 413, "PAYLOAD_TOO_LARGE", "배경 이미지는 8MiB까지 업로드할 수 있습니다.")
			return
		}
		id, err := a.Backgrounds.Save(data)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 201, gin.H{"id": id})
	})
	private.GET("/backgrounds/:id", func(c *gin.Context) {
		data, mime, err := a.Backgrounds.Read(c.Param("id"))
		if err != nil {
			respondError(c, err)
			return
		}
		c.Data(200, mime, data)
	})
}
