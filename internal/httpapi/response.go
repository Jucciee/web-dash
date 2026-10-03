package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"homedashboard/internal/domain"
)

type requestIDKey struct{}

func requestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func success(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data, "meta": gin.H{"requestId": requestID(c.Request.Context())}})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": message}, "meta": map[string]any{"requestId": requestID(r.Context())}})
}

func failure(c *gin.Context, status int, code, message string) {
	writeError(c.Writer, c.Request, status, code, message)
	c.Abort()
}

func respondError(c *gin.Context, err error) {
	var invalid *domain.ValidationError
	var importError *domain.ImportError
	switch {
	case errors.As(err, &importError):
		status := 422
		switch importError.Code {
		case "DOCKER_UNAVAILABLE":
			status = 502
		case "IMPORT_EXPIRED", "IMPORT_COMMITTED", "IMPORT_DUPLICATE":
			status = 409
		case "IMPORT_LIMIT":
			status = 413
		}
		failure(c, status, importError.Code, importError.Message)
	case errors.As(err, &invalid):
		failure(c, 400, "VALIDATION_ERROR", invalid.Message)
	case errors.Is(err, domain.ErrNotFound):
		failure(c, 404, "NOT_FOUND", "요청한 항목을 찾을 수 없습니다.")
	case errors.Is(err, domain.ErrConflict):
		failure(c, 409, "REVISION_CONFLICT", "다른 화면에서 수정된 항목입니다. 최신 내용을 불러온 뒤 다시 저장해주세요.")
	case errors.Is(err, domain.ErrGroupNotEmpty):
		failure(c, 409, "GROUP_NOT_EMPTY", "링크를 다른 그룹으로 이동한 뒤 그룹을 삭제해주세요.")
	case errors.Is(err, domain.ErrAlreadySetup):
		failure(c, 409, "SETUP_COMPLETED", "관리자 설정이 이미 완료되었습니다. 로그인해주세요.")
	case errors.Is(err, domain.ErrUnauthorized):
		failure(c, 401, "UNAUTHORIZED", "인증 정보를 확인해주세요.")
	default:
		slog.Error("request failed", "requestId", requestID(c.Request.Context()), "error", err)
		failure(c, 500, "INTERNAL_ERROR", "요청을 처리하지 못했습니다. 잠시 후 다시 시도해주세요.")
	}
}
