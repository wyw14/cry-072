package httptransport

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/middleware"
)

type errorResponse struct {
	Code        string                  `json:"code"`
	Message     string                  `json:"message"`
	FieldErrors []domain.FieldViolation `json:"field_errors,omitempty"`
	RequestID   string                  `json:"request_id"`
}

func writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	response := errorResponse{Code: "INTERNAL_ERROR", Message: "服务暂时无法完成请求", RequestID: middleware.CurrentRequestID(c)}
	var validationError domain.ValidationError
	var validatorErrors validator.ValidationErrors
	switch {
	case errors.As(err, &validationError):
		status, response.Code, response.Message, response.FieldErrors = http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求字段不符合业务规则", validationError.Violations
	case errors.As(err, &validatorErrors):
		status, response.Code, response.Message = http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求字段格式不正确"
		for _, item := range validatorErrors {
			response.FieldErrors = append(response.FieldErrors, domain.FieldViolation{Field: item.Field(), Message: item.Tag()})
		}
	case errors.Is(err, domain.ErrNotFound):
		status, response.Code, response.Message = http.StatusNotFound, "NOT_FOUND", "请求的资源不存在"
	case errors.Is(err, domain.ErrConflict):
		status, response.Code, response.Message = http.StatusConflict, "VERSION_CONFLICT", "资源已被其他操作更新"
	case errors.Is(err, domain.ErrDuplicate):
		status, response.Code, response.Message = http.StatusConflict, "DUPLICATE", "相同请求或资源已经存在"
	case errors.Is(err, domain.ErrForbidden):
		status, response.Code, response.Message = http.StatusForbidden, "FORBIDDEN", "当前操作人无权执行该操作"
	case errors.Is(err, domain.ErrInvalidTransition):
		status, response.Code, response.Message = http.StatusConflict, "INVALID_TRANSITION", "当前状态不允许该操作"
	}
	c.AbortWithStatusJSON(status, response)
}

func meta(c *gin.Context) application.RequestMeta {
	operatorID, role := middleware.CurrentOperator(c)
	return application.RequestMeta{ActorID: operatorID, Role: role, RequestID: middleware.CurrentRequestID(c)}
}

func expectedVersion(c *gin.Context) (int64, error) {
	value := c.GetHeader("If-Match")
	if value == "" {
		value = c.Query("version")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return 0, domain.NewValidationError("version", "必须通过 If-Match 或 version 提供正整数版本")
	}
	return parsed, nil
}
