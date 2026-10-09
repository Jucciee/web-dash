package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"homedashboard/internal/domain"
)

func bindAddon(c *gin.Context, target any) bool {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			failure(c, 413, "PAYLOAD_TOO_LARGE", "애드온 요청은 64KiB까지 사용할 수 있습니다.")
			return false
		}
		failure(c, 400, "VALIDATION_ERROR", "지원하는 애드온 설정 형식과 입력값을 확인해주세요.")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		failure(c, 400, "VALIDATION_ERROR", "애드온 설정에는 JSON 문서 하나만 넣어주세요.")
		return false
	}
	return true
}
func (a *API) addonRoutes(private *gin.RouterGroup) {
	private.GET("/addons", func(c *gin.Context) {
		result, err := a.Addons.List(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, result)
	})
	private.POST("/addons", func(c *gin.Context) {
		var input domain.AddonWrite
		if !bindAddon(c, &input) {
			return
		}
		result, err := a.Addons.Save(c.Request.Context(), "", 0, input)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 201, result)
	})
	private.POST("/addons/preview", func(c *gin.Context) {
		var input domain.AddonPreview
		if !bindAddon(c, &input) {
			return
		}
		result, err := a.Addons.Preview(c.Request.Context(), input)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, result)
	})
	private.GET("/addons/:id", func(c *gin.Context) {
		result, err := a.Addons.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, result)
	})
	private.PATCH("/addons/:id", func(c *gin.Context) {
		revision, ok := expectedRevision(c)
		if !ok {
			return
		}
		var input domain.AddonWrite
		if !bindAddon(c, &input) {
			return
		}
		result, err := a.Addons.Save(c.Request.Context(), c.Param("id"), revision, input)
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, result)
	})
	private.DELETE("/addons/:id", func(c *gin.Context) {
		revision, ok := expectedRevision(c)
		if !ok {
			return
		}
		if err := a.Addons.Delete(c.Request.Context(), c.Param("id"), revision); err != nil {
			respondError(c, err)
			return
		}
		c.Status(204)
	})
	// CSRF applies to every outbound read, including manifests using POST.
	private.POST("/addons/:id/read", func(c *gin.Context) {
		result, err := a.Addons.Read(c.Request.Context(), c.Param("id"))
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, result)
	})
	private.POST("/addons/:id/move", func(c *gin.Context) {
		revision, ok := expectedRevision(c)
		if !ok {
			return
		}
		var input struct {
			Direction int `json:"direction"`
		}
		if !bindAddon(c, &input) {
			return
		}
		if err := a.Store.MoveAddon(c.Request.Context(), c.Param("id"), revision, input.Direction); err != nil {
			respondError(c, err)
			return
		}
		result, err := a.Addons.List(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
		success(c, 200, result)
	})
}
