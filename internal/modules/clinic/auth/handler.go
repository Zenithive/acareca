package auth

import "github.com/gin-gonic/gin"

type IHandler interface {
	Login(c *gin.Context)
	Register(c *gin.Context)
}

type Handler struct {
}

func NewHandler() IHandler {
	return &Handler{}
}

// Login implements [IHandler].
func (h *Handler) Login(c *gin.Context) {
	panic("unimplemented")
}

// Register implements [IHandler].
func (h *Handler) Register(c *gin.Context) {
	panic("unimplemented")
}
