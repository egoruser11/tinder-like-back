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
	result, err := h.service.GetIncomingLikes(c.Request.Context(), currentUserID(c))
	h.respond(c, result, err)
}

func (h *RibbonHandler) Like(c *gin.Context) {
	var input service.TargetInput
	if !bindJSON(c, &input) {
		return
	}
	h.respond(c, nil, h.service.Like(c.Request.Context(), currentUserID(c), input))
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
	if errors.Is(err, service.ErrNotImplemented) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "ribbon algorithm is not implemented yet"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ribbon operation failed"})
		return
	}
	if result == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, result)
}
