package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"diploma/geolocation-service/internal/repository"

	"github.com/gin-gonic/gin"
)

type GeolocationService interface {
	FindNearbyWorkers(ctx context.Context, categoryID int, latitude float64, longitude float64, radiusMeters int) ([]repository.NearbyWorker, error)
	UpdateWorkerLocation(ctx context.Context, userID int, latitude float64, longitude float64) error
	UpdateCustomerLocation(ctx context.Context, userID int, latitude float64, longitude float64) error
}

type Handler struct {
	geo GeolocationService
}

func New(geo GeolocationService) *Handler {
	return &Handler{geo: geo}
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) FindNearbyWorkers(c *gin.Context) {
	categoryID, err := parseIntQuery(c, "category_id", true, 0)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	latitude, err := parseFloatQuery(c, "latitude")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	longitude, err := parseFloatQuery(c, "longitude")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	radiusMeters, err := parseIntQuery(c, "radius_meters", false, 5000)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workers, err := h.geo.FindNearbyWorkers(
		c.Request.Context(),
		categoryID,
		latitude,
		longitude,
		radiusMeters,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, workers)
}

func (h *Handler) UpdateWorkerLocation(c *gin.Context) {
	if c.GetString("role") != "worker" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only workers allowed"})
		return
	}
	h.updateLocation(c, h.geo.UpdateWorkerLocation)
}

func (h *Handler) UpdateCustomerLocation(c *gin.Context) {
	if c.GetString("role") != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only customers allowed"})
		return
	}
	h.updateLocation(c, h.geo.UpdateCustomerLocation)
}

func (h *Handler) updateLocation(
	c *gin.Context,
	update func(context.Context, int, float64, float64) error,
) {
	var req struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	if err := update(c.Request.Context(), c.GetInt("user_id"), req.Latitude, req.Longitude); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "location updated"})
}

func parseIntQuery(c *gin.Context, key string, required bool, fallback int) (int, error) {
	raw := c.Query(key)
	if raw == "" {
		if required {
			return 0, errors.New(key + " is required")
		}
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid " + key)
	}
	return value, nil
}

func parseFloatQuery(c *gin.Context, key string) (float64, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, errors.New(key + " is required")
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, errors.New("invalid " + key)
	}
	return value, nil
}
