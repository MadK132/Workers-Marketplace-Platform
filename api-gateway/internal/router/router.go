package router

import (
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"

	"diploma/api-gateway/internal/config"
	"diploma/api-gateway/internal/middleware"
	"diploma/api-gateway/internal/proxy"

	"github.com/gin-gonic/gin"
)

func Setup(cfg config.Config, userProxy, bookingProxy *httputil.ReverseProxy) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(middleware.RequestID())
	r.Use(gin.Logger())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.Any("/auth/*path", func(c *gin.Context) {
		proxy.StripTrustedHeaders(c.Request)
		userProxy.ServeHTTP(c.Writer, c.Request)
	})

	api := r.Group("/api")
	api.Use(middleware.Auth(cfg.JWTSecret))
	api.Any("/*path", func(c *gin.Context) {
		path := c.Param("path")
		if strings.HasPrefix(path, "/internal/") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "internal endpoints are not exposed by gateway",
			})
			return
		}

		userID := c.GetInt("user_id")
		role := c.GetString("role")

		c.Request.Header.Set("X-User-ID", strconv.Itoa(userID))
		c.Request.Header.Set("X-Role", role)
		if cfg.GatewaySecret != "" {
			c.Request.Header.Set("X-Gateway-Secret", cfg.GatewaySecret)
		}

		if isBookingPath(path) {
			bookingProxy.ServeHTTP(c.Writer, c.Request)
			return
		}
		userProxy.ServeHTTP(c.Writer, c.Request)
	})

	return r
}

func isBookingPath(path string) bool {
	return strings.HasPrefix(path, "/requests") || strings.HasPrefix(path, "/bookings")
}
