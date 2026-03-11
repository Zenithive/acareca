package version

import "github.com/gin-gonic/gin"

type IHandler interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	List(c *gin.Context)
}

type handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &handler{svc: svc}
}

// Create implements [IHandler].
func (h *handler) Create(c *gin.Context) {
	panic("unimplemented")
}

// Delete implements [IHandler].
func (h *handler) Delete(c *gin.Context) {
	panic("unimplemented")
}

// Get implements [IHandler].
func (h *handler) Get(c *gin.Context) {
	panic("unimplemented")
}

// Update implements [IHandler].
func (h *handler) Update(c *gin.Context) {
	panic("unimplemented")
}

// List implements [IHandler].
func (h *handler) List(c *gin.Context) {
	panic("unimplemented")
}
