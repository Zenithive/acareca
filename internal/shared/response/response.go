package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RsError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

type envelope struct {
	Status  int      `json:"status,omitempty"`
	Data    any      `json:"data,omitempty"`
	Message string   `json:"message,omitempty"`
	Error   *RsError `json:"error,omitempty"`
}

func JSON(c *gin.Context, status int, data any, message string) {
	c.JSON(status, envelope{
		Status:  status,
		Data:    data,
		Message: message,
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
	} else {
		msg = "unknown error"
	}

	details := ""
	if err != nil {
		details = err.Error()
	}

	c.AbortWithStatusJSON(status, envelope{
		Error: &RsError{
			Code:    status,
			Message: msg,
			Detail:  details,
		},
	})
}

func NewUUID() string {
	return uuid.New().String()
}
