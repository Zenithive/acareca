package file

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Handler interface {
	GetDocument(c *gin.Context)
	ListDocuments(c *gin.Context)
	UpdateDocument(c *gin.Context)
	DeleteDocument(c *gin.Context)
	Upload(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) Handler {
	return &handler{svc: svc}
}

// GetDocument godoc
// @Summary Get document metadata
// @Description Get metadata for a specific document
// @Tags files
// @Produce json
// @Security BearerToken
// @Param id path string true "Document ID (UUID)"
// @Success 200 {object} response.RsBase{data=RsDocument}
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Router /files/{id} [get]
func (h *handler) GetDocument(c *gin.Context) {
	// Get user info
	userIDPtr := auditctx.GetUserID(c.Request.Context())
	if userIDPtr == nil {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: user not found"))
		return
	}

	userID, err := uuid.Parse(*userIDPtr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}

	// Get document ID
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid document id"))
		return
	}

	// Get document
	doc, err := h.svc.GetDocument(c.Request.Context(), docID, userID)
	if err != nil {
		if errors.Is(err, ErrDocumentNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, ErrUnauthorizedAccess) {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, doc, "Document retrieved successfully")
}

// ListDocuments godoc
// @Summary List documents
// @Description List documents owned by the authenticated user
// @Tags files
// @Produce json
// @Security BearerToken
// @Param entity_type query string false "Filter by entity type"
// @Param entity_id query string false "Filter by entity ID"
// @Param status query string false "Filter by status"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Param sort query string false "Sort field" default(created_at)
// @Param order query string false "Sort order (asc/desc)" default(desc)
// @Success 200 {object} response.RsBase{data=RsListDocuments}
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Router /files [get]
func (h *handler) ListDocuments(c *gin.Context) {
	// Get user info
	userIDPtr := auditctx.GetUserID(c.Request.Context())
	if userIDPtr == nil {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: user not found"))
		return
	}

	userID, err := uuid.Parse(*userIDPtr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}

	// Parse query parameters
	var filters RqListDocuments
	if err := c.ShouldBindQuery(&filters); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// List documents
	result, err := h.svc.ListDocuments(c.Request.Context(), userID, &filters)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Documents retrieved successfully")
}

// UpdateDocument godoc
// @Summary Update document metadata
// @Description Update metadata for a specific document
// @Tags files
// @Accept json
// @Produce json
// @Security BearerToken
// @Param id path string true "Document ID (UUID)"
// @Param request body RqUpdateDocument true "Update data"
// @Success 200 {object} response.RsBase{data=RsDocument}
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Router /files/{id} [put]
func (h *handler) UpdateDocument(c *gin.Context) {
	// Get user info
	userIDPtr := auditctx.GetUserID(c.Request.Context())
	if userIDPtr == nil {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: user not found"))
		return
	}

	userID, err := uuid.Parse(*userIDPtr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}

	// Get document ID
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid document id"))
		return
	}

	// Parse request body
	var req RqUpdateDocument
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Update document
	doc, err := h.svc.UpdateDocument(c.Request.Context(), docID, &req, userID)
	if err != nil {
		if errors.Is(err, ErrDocumentNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, ErrUnauthorizedAccess) {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, doc, "Document updated successfully")
}

// DeleteDocument godoc
// @Summary Delete a document
// @Description Soft delete a document
// @Tags files
// @Produce json
// @Security BearerToken
// @Param id path string true "Document ID (UUID)"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Router /files/{id} [delete]
func (h *handler) DeleteDocument(c *gin.Context) {
	// Get user info
	userIDPtr := auditctx.GetUserID(c.Request.Context())
	if userIDPtr == nil {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: user not found"))
		return
	}

	userID, err := uuid.Parse(*userIDPtr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}

	// Get document ID
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid document id"))
		return
	}

	// Delete document
	if err := h.svc.DeleteDocument(c.Request.Context(), docID, userID); err != nil {
		if errors.Is(err, ErrDocumentNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, ErrUnauthorizedAccess) {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Document deleted successfully")
}

// Upload godoc
// @Summary Generate presigned upload URL
// @Description Generate a presigned URL for direct upload to R2 storage
// @Tags files
// @Accept json
// @Produce json
// @Security BearerToken
// @Param request body RqGeneratePresignedUploadURL true "Presigned URL parameters"
// @Success 200 {object} response.RsBase{data=RsPresignedUploadURL}
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /files/presigned-upload [post]
func (h *handler) Upload(c *gin.Context) {
	// Get user info
	userIDPtr := auditctx.GetUserID(c.Request.Context())
	if userIDPtr == nil {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: user not found"))
		return
	}

	userID, err := uuid.Parse(*userIDPtr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}

	// Get user role
	rolePtr := auditctx.GetUserType(c.Request.Context())
	if rolePtr == nil {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized: role not found"))
		return
	}

	var req RqGeneratePresignedUploadURL
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("file is required"))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, errors.New("failed to open file"))
		return
	}
	defer file.Close()

	// Generate presigned URL
	result, err := h.svc.GeneratePresignedUploadURL(c.Request.Context(), &req, userID, *rolePtr, file, fileHeader)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Presigned upload URL generated successfully")
}
