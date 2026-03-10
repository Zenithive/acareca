package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RsError struct {
	Message string `json:"message" example:"internal server error"`
	Code    int    `json:"code" example:"500"`
}

type envelope struct {
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	RsError `json:"error"`
}

func JSON(c *gin.Context, status int, data any) {
	c.JSON(status, envelope{Data: data})
}

func Message(c *gin.Context, status int, message string) {
	c.JSON(status, envelope{Message: message})
}

func Error(c *gin.Context, status int, err error) {
	msg := "internal server error"
	if status < http.StatusInternalServerError {
		msg = err.Error()
	}
	c.AbortWithStatusJSON(status, envelope{RsError: RsError{Message: msg, Code: status}})
}

func NewUUID() string {
	return uuid.New().String()
}
