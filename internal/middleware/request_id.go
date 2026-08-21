package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const RequestIDKey = "request_id"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 {
			bytes := make([]byte, 12)
			if _, err := rand.Read(bytes); err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": "REQUEST_ID_FAILURE", "message": "无法生成请求标识"})
				return
			}
			requestID = hex.EncodeToString(bytes)
		}
		c.Set(RequestIDKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func CurrentRequestID(c *gin.Context) string {
	value, _ := c.Get(RequestIDKey)
	requestID, _ := value.(string)
	return requestID
}
