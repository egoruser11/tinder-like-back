package handlers

import (
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"tinder-core/internal/repository"
	"tinder-core/internal/service"
	transportmiddleware "tinder-core/internal/transport/http/middleware"
)

type PhotoHandler struct {
	service *service.PhotoService
}

func NewPhotoHandler(service *service.PhotoService) *PhotoHandler {
	return &PhotoHandler{service: service}
}

func (h *PhotoHandler) ListMe(c *gin.Context) {
	photos, err := h.service.List(c.Request.Context(), currentPhotoUserID(c))
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": photos})
}

func (h *PhotoHandler) UploadMe(c *gin.Context) {
	fileHeader, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "photo multipart field is required"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read photo"})
		return
	}
	defer file.Close()

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(fileHeader.Filename))
	}
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	photo, err := h.service.Upload(c.Request.Context(), currentPhotoUserID(c), contentType, fileHeader.Size, file)
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, photo)
}

func (h *PhotoHandler) DeleteMe(c *gin.Context) {
	photoID, err := strconv.ParseInt(c.Param("photo_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": service.ErrInvalidPhotoID.Error()})
		return
	}
	if err := h.service.Delete(c.Request.Context(), currentPhotoUserID(c), photoID); err != nil {
		h.respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func currentPhotoUserID(c *gin.Context) int64 {
	return c.GetInt64(transportmiddleware.UserIDKey)
}

func (h *PhotoHandler) respond(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidPhoto), errors.Is(err, service.ErrInvalidPhotoID):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrPhotoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "photo operation failed"})
	}
}
