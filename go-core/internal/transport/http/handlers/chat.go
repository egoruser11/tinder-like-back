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

type ChatHandler struct {
	service *service.ChatService
}

func NewChatHandler(service *service.ChatService) *ChatHandler {
	return &ChatHandler{service: service}
}

func (h *ChatHandler) List(c *gin.Context) {
	result, err := h.service.List(c.Request.Context(), currentChatUserID(c), chatPageInput(c))
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ChatHandler) Messages(c *gin.Context) {
	chatID, ok := parseChatID(c)
	if !ok {
		return
	}
	result, err := h.service.Messages(c.Request.Context(), currentChatUserID(c), chatID, chatPageInput(c))
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ChatHandler) Send(c *gin.Context) {
	chatID, ok := parseChatID(c)
	if !ok {
		return
	}
	var request struct {
		Body string `json:"body"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message body"})
		return
	}
	message, err := h.service.Send(c.Request.Context(), currentChatUserID(c), chatID, request.Body)
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, message)
}

func (h *ChatHandler) MarkRead(c *gin.Context) {
	chatID, ok := parseChatID(c)
	if !ok {
		return
	}
	if err := h.service.MarkRead(c.Request.Context(), currentChatUserID(c), chatID); err != nil {
		h.respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func currentChatUserID(c *gin.Context) int64 {
	return c.GetInt64(transportmiddleware.UserIDKey)
}

func chatPageInput(c *gin.Context) service.ChatPageInput {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	return service.ChatPageInput{Limit: limit, Cursor: c.Query("cursor")}
}

func parseChatID(c *gin.Context) (int64, bool) {
	chatID, err := strconv.ParseInt(c.Param("chat_id"), 10, 64)
	if err != nil || chatID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": service.ErrInvalidChatID.Error()})
		return 0, false
	}
	return chatID, true
}

func (h *ChatHandler) respond(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidChatID),
		errors.Is(err, service.ErrInvalidMessageBody),
		errors.Is(err, service.ErrInvalidChatCursor):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrChatNotFound):
		// The same answer for a missing chat and a chat belonging to another user
		// avoids exposing which chat IDs exist.
		c.JSON(http.StatusNotFound, gin.H{"error": "chat not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "chat operation failed"})
	}
}
