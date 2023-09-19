package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const requestIDKey = "request_id"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if id == "" {
			id = newID()
		}
		c.Set(requestIDKey, id)
		c.Writer.Header().Set("X-Request-ID", id)
		c.Next()
	}
}

func RequestIDFrom(c *gin.Context) string {
	raw, ok := c.Get(requestIDKey)
	if !ok {
		return ""
	}
	id, _ := raw.(string)
	return id
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
