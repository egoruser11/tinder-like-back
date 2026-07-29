package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/meysam81/go-auth/auth/basic"
	authjwt "github.com/meysam81/go-auth/auth/jwt"
	"github.com/meysam81/go-auth/storage"

	transportmiddleware "tinder-core/internal/transport/http/middleware"
)

type AuthHandler struct {
	authenticator *basic.Authenticator
	tokenManager  *authjwt.TokenManager
	logger        *slog.Logger
}

func NewAuthHandler(authenticator *basic.Authenticator, tokenManager *authjwt.TokenManager, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authenticator: authenticator,
		tokenManager:  tokenManager,
		logger:        logger,
	}
}

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type authResponse struct {
	AccessToken string  `json:"access_token"`
	TokenType   string  `json:"token_type"`
	User        userDTO `json:"user"`
}

type userDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password (minimum 8 characters) are required"})
		return
	}

	user, err := h.authenticator.Register(c.Request.Context(), basic.RegisterRequest{
		Email:    strings.ToLower(strings.TrimSpace(request.Email)),
		Password: request.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, basic.ErrUserExists), errors.Is(err, storage.ErrAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{"error": "email is already registered"})
		case errors.Is(err, basic.ErrWeakPassword):
			c.JSON(http.StatusBadRequest, gin.H{"error": "password must contain at least 8 characters"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not register user"})
		}
		return
	}

	h.logger.Info("user registered", "user_id", user.ID)
	h.respondWithAccessToken(c, http.StatusCreated, user)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	user, err := h.authenticator.Authenticate(
		c.Request.Context(),
		strings.ToLower(strings.TrimSpace(request.Email)),
		request.Password,
	)
	if err != nil {
		if errors.Is(err, basic.ErrInvalidCredentials) {
			h.logger.Warn("login failed", "reason", "invalid_credentials")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not authenticate user"})
		return
	}

	h.logger.Info("user logged in", "user_id", user.ID)
	h.respondWithAccessToken(c, http.StatusOK, user)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetInt64(transportmiddleware.UserIDKey)
	email := c.GetString(transportmiddleware.UserEmailKey)
	c.JSON(http.StatusOK, userDTO{ID: strconv.FormatInt(userID, 10), Email: email})
}

func (h *AuthHandler) respondWithAccessToken(c *gin.Context, status int, user *storage.User) {
	token, err := h.tokenManager.GenerateAccessToken(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create access token"})
		return
	}

	c.JSON(status, authResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		User:        userDTO{ID: user.ID, Email: user.Email},
	})
}
