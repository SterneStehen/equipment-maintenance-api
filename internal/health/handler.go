// Package health provides the service health endpoint.
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler reports that the HTTP process is available.
func Handler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
