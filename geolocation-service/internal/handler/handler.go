package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"diploma/geolocation-service/internal/repository"
	"diploma/internal/notifications"

	"github.com/gin-gonic/gin"
)

type GeolocationService interface {
	FindNearbyWorkers(ctx context.Context, categoryID int, latitude float64, longitude float64, radiusMeters int) ([]repository.NearbyWorker, error)
}

type Handler struct {
	geo      GeolocationService
	notifier notifications.Publisher
}

func New(geo GeolocationService, notifier notifications.Publisher) *Handler {
	return &Handler{geo: geo, notifier: notifier}
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
	if len(workers) == 0 && c.GetInt("user_id") > 0 {
		_ = h.notifier.Publish(c.Request.Context(), notifications.Event{
			SourceService:   "geolocation-service",
			Type:            "geolocation.no_workers_found",
			RecipientUserID: int64(c.GetInt("user_id")),
			Title:           "No nearby workers found",
			Body:            "No available workers were found for your selected category and radius.",
			Data: map[string]any{
				"category_id":   categoryID,
				"latitude":      latitude,
				"longitude":     longitude,
				"radius_meters": radiusMeters,
			},
		})
	}

	c.JSON(http.StatusOK, workers)
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
