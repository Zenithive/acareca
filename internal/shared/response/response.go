package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type envelope struct {
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
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
	c.AbortWithStatusJSON(status, envelope{Error: msg})
}

func NewUUID() string {
	return uuid.New().String()
}
