package middleware

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONLoggerIncludesRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(RequestID(), JSONLogger(logger))
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusAccepted, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-ID", "abc")
	router.ServeHTTP(httptest.NewRecorder(), req)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &body))
	assert.Equal(t, "abc", body["request_id"])
	assert.Equal(t, "GET", body["method"])
	assert.Equal(t, "/ping", body["path"])
	assert.Equal(t, float64(http.StatusAccepted), body["status"])
}
