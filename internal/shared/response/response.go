package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type envelope struct {
	Status  int          `json:"status,omitempty"`
	Data    any          `json:"data,omitempty"`
	Message string       `json:"message,omitempty"`
	Error   *ErrorDetail `json:"error,omitempty"`
}

func JSON(c *gin.Context, status int, data any) {
	c.JSON(status, envelope{
		Status: status,
		Data:   data,
	})
}

func Message(c *gin.Context, status int, message string) {
	c.JSON(status, envelope{
		Status:  status,
		Message: message,
	})
}

func Error(c *gin.Context, status int, err error) {
	var msg string
	if status >= http.StatusInternalServerError {
		msg = "internal server error"
	} else if err != nil {
		msg = err.Error()
	} else {
		msg = "unknown error"
	}

	c.AbortWithStatusJSON(status, envelope{
		Error: &ErrorDetail{
			Code:    status,
			Message: msg,
		},
	})
}

func NewUUID() string {
	return uuid.New().String()
}
