package http

import (
	"github.com/gin-gonic/gin"

	"tinder-core/internal/events"
	"tinder-core/internal/transport/http/handlers"
)

type Deps struct {
	Publisher *events.Publisher
}

func NewRouter(deps Deps) *gin.Engine {
	r := gin.Default()

	r.GET("/health", handlers.Health)

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		_ = auth // TODO(ticket-3): POST /register, POST /login, POST /refresh

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
