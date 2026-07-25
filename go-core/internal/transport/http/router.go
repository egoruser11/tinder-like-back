package http

import (
	"github.com/gin-gonic/gin"
	"github.com/meysam81/go-auth/auth/basic"
	authjwt "github.com/meysam81/go-auth/auth/jwt"

	"tinder-core/internal/events"
	"tinder-core/internal/transport/http/handlers"
	transportmiddleware "tinder-core/internal/transport/http/middleware"
)

type Deps struct {
	Publisher     *events.Publisher
	Authenticator *basic.Authenticator
	TokenManager  *authjwt.TokenManager
}

func NewRouter(deps Deps) *gin.Engine {
	r := gin.Default()

	r.GET("/health", handlers.Health)
	authHandler := handlers.NewAuthHandler(deps.Authenticator, deps.TokenManager)

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)

		protected := v1.Group("")
		protected.Use(transportmiddleware.RequireJWT(deps.TokenManager))
		protected.GET("/auth/me", authHandler.Me)

		profiles := v1.Group("/profiles")
		_ = profiles // TODO(ticket-4): GET/PUT /profiles/me, POST /profiles/me/photos

		feed := v1.Group("/feed")
		_ = feed // TODO(ticket-6): GET /feed (candidate stack, cached in Redis)

		swipes := v1.Group("/swipes")
		_ = swipes // TODO(ticket-5): POST /swipes (like/pass -> match detection -> events.Publish)

		matches := v1.Group("/matches")
		_ = matches // TODO(ticket-5): GET /matches
	}

	return r
}
