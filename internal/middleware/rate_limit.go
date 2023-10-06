package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/SterneStehen/equipment-maintenance-api/internal/apperror"
	"github.com/gin-gonic/gin"
)

type hitBucket struct {
	start time.Time
	n     int
}

func RateLimit(maxHits int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	buckets := map[string]hitBucket{}
	if maxHits < 1 {
		maxHits = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return func(c *gin.Context) {
		key := c.ClientIP() + " " + c.FullPath()
		now := time.Now()
		mu.Lock()
		b := buckets[key]
		if b.start.IsZero() || now.Sub(b.start) >= window {
			b = hitBucket{start: now}
		}
		b.n++
		buckets[key] = b
		block := b.n > maxHits
		mu.Unlock()

		if block {
			c.Header("Retry-After", retryAfter(window))
			apperror.Write(c, http.StatusTooManyRequests, "rate_limited", "Too many authentication attempts")
			return
		}
		c.Next()
	}
}

func retryAfter(window time.Duration) string {
	secs := int(window.Seconds())
	if secs < 1 {
		secs = 1
	}
	return strconv.Itoa(secs)
}
