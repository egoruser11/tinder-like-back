package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

var ErrProfileNotFound = errors.New("profile not found")

// ProfileDeactivator — минимальный контракт handler-а с репозиторием.
// Реализация появится в internal/repository в рамках тикета 2.
type ProfileDeactivator interface {
	Deactivate(ctx context.Context, userID int64) error
}

// ProfileHandler содержит зависимости HTTP-слоя.
// Handler не должен сам выполнять SQL-запросы.
type ProfileHandler struct {
	profiles ProfileDeactivator
}

func NewProfileHandler(profiles ProfileDeactivator) *ProfileHandler {
	return &ProfileHandler{profiles: profiles}
}

// DeleteMe — пример handler-а мягкого удаления профиля.
// auth middleware из тикета 3 должен положить идентификатор пользователя
// в контекст Gin под ключом "user_id".
func (h *ProfileHandler) DeleteMe(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	err := h.profiles.Deactivate(c.Request.Context(), userID)
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, ErrProfileNotFound):
		c.JSON(http.StatusNotFound, map[string]string{"error": "profile not found"})
	default:
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "could not delete profile"})
	}
}
