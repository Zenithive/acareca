package invoice

import "github.com/gin-gonic/gin"

type IHandler interface {
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
}

type Handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &Handler{
		svc: svc,
	}
}

// Create implements [IHandler].
func (h *Handler) Create(c *gin.Context) {

}

// Delete implements [IHandler].
func (h *Handler) Delete(c *gin.Context) {
	panic("unimplemented")
}

// Get implements [IHandler].
func (h *Handler) Get(c *gin.Context) {
	panic("unimplemented")
}

// List implements [IHandler].
func (h *Handler) List(c *gin.Context) {
	panic("unimplemented")
}

// Update implements [IHandler].
func (h *Handler) Update(c *gin.Context) {
	panic("unimplemented")
}
