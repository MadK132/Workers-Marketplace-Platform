package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = strconv.FormatInt(time.Now().UnixNano(), 10)
		}
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Request.Header.Set("X-Request-ID", requestID)
		c.Next()
	}
}
