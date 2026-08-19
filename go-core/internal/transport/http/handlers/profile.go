package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"tinder-core/internal/repository"
	"tinder-core/internal/service"
	transportmiddleware "tinder-core/internal/transport/http/middleware"
)

// ProfileHandler owns the HTTP contract for the current user's profile.
type ProfileHandler struct {
	service *service.ProfileService
}

func NewProfileHandler(service *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{service: service}
}

func (h *ProfileHandler) Me(c *gin.Context) {
	profile, err := h.service.Get(c.Request.Context(), currentProfileUserID(c))
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *ProfileHandler) SaveMe(c *gin.Context) {
	var input service.SaveProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile body"})
		return
	}

	profile, err := h.service.Save(c.Request.Context(), currentProfileUserID(c), input)
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *ProfileHandler) DeleteMe(c *gin.Context) {
	err := h.service.Deactivate(c.Request.Context(), currentProfileUserID(c))
	if err != nil {
		h.respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func currentProfileUserID(c *gin.Context) int64 {
	return c.GetInt64(transportmiddleware.UserIDKey)
}

func (h *ProfileHandler) respond(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrProfileNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrProfileNameRequired),
		errors.Is(err, service.ErrInvalidProfileBirthday),
		errors.Is(err, service.ErrProfileOwnerMustBeAdult),
		errors.Is(err, service.ErrInvalidProfileGender),
		errors.Is(err, service.ErrCoordinatesMustBeTogether),
		errors.Is(err, service.ErrInvalidProfileCoordinates):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "profile operation failed"})
	}
}
