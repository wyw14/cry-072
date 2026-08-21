package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wyw14/cry-072/internal/domain"
)

const (
	OperatorIDKey   = "operator_id"
	OperatorRoleKey = "operator_role"
)

func Operator(defaultID string, defaultRole domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		operatorID := strings.TrimSpace(c.GetHeader("X-Operator-ID"))
		role := domain.ParseRole(c.GetHeader("X-Operator-Role"))
		if operatorID == "" {
			operatorID = defaultID
		}
		if role == "" {
			role = defaultRole
		}
		if operatorID == "" || !role.Valid() {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": "INVALID_OPERATOR", "message": "操作人身份无效", "request_id": CurrentRequestID(c),
			})
			return
		}
		c.Set(OperatorIDKey, operatorID)
		c.Set(OperatorRoleKey, role)
		c.Next()
	}
}

func CurrentOperator(c *gin.Context) (string, domain.Role) {
	idValue, _ := c.Get(OperatorIDKey)
	roleValue, _ := c.Get(OperatorRoleKey)
	id, _ := idValue.(string)
	role, _ := roleValue.(domain.Role)
	return id, role
}
