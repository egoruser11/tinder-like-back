package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"tinder-core/internal/repository"
	"tinder-core/internal/service"
	transportmiddleware "tinder-core/internal/transport/http/middleware"
)

// RibbonHandler owns only HTTP concerns. Candidate selection and all database
// mutations belong in service.RibbonService.
type RibbonHandler struct {
	service    *service.RibbonService
	repository *repository.RibbonRepository
}

func NewRibbonHandler(service *service.RibbonService, repository *repository.RibbonRepository) *RibbonHandler {
	return &RibbonHandler{service: service, repository: repository}
}

func (h *RibbonHandler) Feed(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	result, err := h.service.GetFeed(c.Request.Context(), currentUserID(c), service.FeedInput{
		Limit:  limit,
		Cursor: c.Query("cursor"),
	})
	h.respond(c, result, err)
}

func (h *RibbonHandler) IncomingLikes(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	result, err := h.service.GetIncomingLikes(c.Request.Context(), currentUserID(c), service.FeedInput{
		Limit:  limit,
		Cursor: c.Query("cursor"),
	})
	h.respond(c, result, err)
}

func (h *RibbonHandler) Preferences(c *gin.Context) {
	preferences, err := h.service.GetPreferences(c.Request.Context(), currentUserID(c))
	if err != nil {
		h.respond(c, nil, err)
		return
	}
	c.JSON(http.StatusOK, preferences)
}

func (h *RibbonHandler) SavePreferences(c *gin.Context) {
	var input service.SavePreferencesInput
	if !bindJSON(c, &input) {
		return
	}
	preferences, err := h.service.SavePreferences(c.Request.Context(), currentUserID(c), input)
	if err != nil {
		h.respond(c, nil, err)
		return
	}
	c.JSON(http.StatusOK, preferences)
}

func (h *RibbonHandler) Like(c *gin.Context) {
	var input service.TargetInput
	if !bindJSON(c, &input) {
		return
	}
	result, err := h.service.Like(c.Request.Context(), currentUserID(c), input)
	h.respond(c, result, err)
}

func (h *RibbonHandler) Dislike(c *gin.Context) {
	var input service.TargetInput
	if !bindJSON(c, &input) {
		return
	}
	h.respond(c, nil, h.service.Dislike(c.Request.Context(), currentUserID(c), input))
}

func (h *RibbonHandler) Block(c *gin.Context) {
	var input service.TargetInput
	if !bindJSON(c, &input) {
		return
	}
	h.respond(c, nil, h.service.Block(c.Request.Context(), currentUserID(c), input))
}

func (h *RibbonHandler) Unblock(c *gin.Context) {
	var input service.TargetInput
	if !bindJSON(c, &input) {
		return
	}
	h.respond(c, nil, h.service.Unblock(c.Request.Context(), currentUserID(c), input))
}

func (h *RibbonHandler) Report(c *gin.Context) {
	var input service.ReportInput
	if !bindJSON(c, &input) {
		return
	}
	h.respond(c, nil, h.service.Report(c.Request.Context(), currentUserID(c), input))
}

func currentUserID(c *gin.Context) int64 {
	return c.GetInt64(transportmiddleware.UserIDKey)
}

func bindJSON(c *gin.Context, value any) bool {
	if err := c.ShouldBindJSON(value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}
	return true
}

func (h *RibbonHandler) respond(c *gin.Context, result any, err error) {
	switch {
	case errors.Is(err, service.ErrNotImplemented):
		c.JSON(http.StatusNotImplemented, gin.H{"error": "ribbon algorithm is not implemented yet"})
		return
	case errors.Is(err, service.ErrInvalidFeedCursor),
		errors.Is(err, service.ErrInvalidFeedLimit),
		errors.Is(err, service.ErrInvalidTargetUser),
		errors.Is(err, service.ErrInvalidReportReason),
		errors.Is(err, service.ErrReportCommentTooLong),
		errors.Is(err, service.ErrInvalidDiscoveryAge),
		errors.Is(err, service.ErrInvalidDiscoveryGender),
		errors.Is(err, service.ErrInvalidDiscoveryDistance):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	case errors.Is(err, repository.ErrTargetUserNotFound), errors.Is(err, repository.ErrProfileNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	case errors.Is(err, repository.ErrDiscoveryPreferencesNotFound),
		errors.Is(err, service.ErrProfileCoordinatesRequired),
		errors.Is(err, repository.ErrUsersExcluded):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ribbon operation failed"})
		return
	}
	if result == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, result)
}
