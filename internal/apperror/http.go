package apperror

import "github.com/gin-gonic/gin"

type envelope struct {
	Error detail `json:"error"`
}

type detail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Write(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, envelope{Error: detail{Code: code, Message: message}})
}
