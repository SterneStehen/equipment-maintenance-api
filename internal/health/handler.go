package health

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type ReadyHandler struct {
	db Pinger
}

func Check(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func NewReadyHandler(db Pinger) *ReadyHandler {
	return &ReadyHandler{db: db}
}

func (h *ReadyHandler) Check(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	if err := h.db.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
