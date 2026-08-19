package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/meysam81/go-auth/auth/basic"
	authjwt "github.com/meysam81/go-auth/auth/jwt"

	"tinder-core/internal/events"
	"tinder-core/internal/transport/http/handlers"
	transportmiddleware "tinder-core/internal/transport/http/middleware"
)

type Deps struct {
	Publisher      *events.Publisher
	Authenticator  *basic.Authenticator
	TokenManager   *authjwt.TokenManager
	Logger         *slog.Logger
	AuthHandler    *handlers.AuthHandler
	RibbonHandler  *handlers.RibbonHandler
	ProfileHandler *handlers.ProfileHandler
}

func NewRouter(deps Deps) *gin.Engine {
	r := gin.Default()

	r.GET("/health", handlers.Health)

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		auth.POST("/register", deps.AuthHandler.Register)
		auth.POST("/login", deps.AuthHandler.Login)

		protected := v1.Group("")
		protected.Use(transportmiddleware.RequireJWT(deps.TokenManager))
		protected.GET("/auth/me", deps.AuthHandler.Me)

		ribbon := protected.Group("/ribbon")
		ribbon.GET("", deps.RibbonHandler.Feed)
		ribbon.GET("/likes", deps.RibbonHandler.IncomingLikes)
		ribbon.GET("/preferences", deps.RibbonHandler.Preferences)
		ribbon.PUT("/preferences", deps.RibbonHandler.SavePreferences)
		ribbon.POST("/likes", deps.RibbonHandler.Like)
		ribbon.POST("/dislikes", deps.RibbonHandler.Dislike)
		ribbon.POST("/blocks", deps.RibbonHandler.Block)
		ribbon.DELETE("/blocks", deps.RibbonHandler.Unblock)
		ribbon.POST("/reports", deps.RibbonHandler.Report)

		profiles := protected.Group("/profiles")
		profiles.GET("/me", deps.ProfileHandler.Me)
		profiles.PUT("/me", deps.ProfileHandler.SaveMe)
		profiles.DELETE("/me", deps.ProfileHandler.DeleteMe)

		feed := v1.Group("/feed")
		_ = feed // TODO(ticket-6): GET /feed (candidate stack, cached in Redis)

		swipes := v1.Group("/swipes")
		_ = swipes // TODO(ticket-5): POST /swipes (like/pass -> match detection -> events.Publish)

		matches := v1.Group("/matches")
		_ = matches // TODO(ticket-5): GET /matches
	}

	return r
}
