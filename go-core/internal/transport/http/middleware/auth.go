package middleware

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	authjwt "github.com/meysam81/go-auth/auth/jwt"
	authmiddleware "github.com/meysam81/go-auth/middleware"
)

const (
	UserIDKey    = "user_id"
	UserEmailKey = "user_email"
)

// RequireJWT adapts go-auth's framework-agnostic net/http middleware to Gin.
// It validates Authorization: Bearer <access-token> and exposes the trusted
// identity to handlers through Gin's context.
func RequireJWT(tokenManager *authjwt.TokenManager) gin.HandlerFunc {
	jwtMiddleware := authmiddleware.NewJWTMiddleware(authmiddleware.JWTConfig{
		TokenManager: tokenManager,
		ErrorHandler: jsonAuthError,
	})

	return func(c *gin.Context) {
		authenticated := false
		protected := jwtMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := authmiddleware.GetClaims(r)
			if !ok {
				jsonAuthError(w, r, authmiddleware.ErrUnauthorized)
				return
			}

			userID, err := strconv.ParseInt(claims.UserID, 10, 64)
			if err != nil || userID < 1 {
				jsonAuthError(w, r, authmiddleware.ErrUnauthorized)
				return
			}

			c.Request = r
			c.Set(UserIDKey, userID)
			c.Set(UserEmailKey, claims.Email)
			authenticated = true
		}))

		protected.ServeHTTP(c.Writer, c.Request)
		if !authenticated {
			c.Abort()
		}
	}
}

func jsonAuthError(w http.ResponseWriter, _ *http.Request, _ error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(gin.H{"error": "unauthorized"})
}
