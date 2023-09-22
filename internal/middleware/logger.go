package middleware

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

type logLine struct {
	Time      string  `json:"time"`
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	Method    string  `json:"method"`
	Path      string  `json:"path"`
	Status    int     `json:"status"`
	LatencyMS float64 `json:"latency_ms"`
	RequestID string  `json:"request_id"`
	ClientIP  string  `json:"client_ip"`
}

func JSONLogger(logger *log.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = log.Default()
	}
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()

		line := logLine{
			Time:      time.Now().UTC().Format(time.RFC3339Nano),
			Level:     "info",
			Message:   "http_request",
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Status:    c.Writer.Status(),
			LatencyMS: float64(time.Since(started).Microseconds()) / 1000,
			RequestID: RequestIDFrom(c),
			ClientIP:  c.ClientIP(),
		}
		raw, err := json.Marshal(line)
		if err != nil {
			logger.Printf(`{"level":"error","message":"log encode failed"}`)
			return
		}
		logger.Print(string(raw))
	}
}
