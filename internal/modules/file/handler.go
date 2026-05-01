package file

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/upload"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Handler interface {
	UploadFile(c *gin.Context)
	UploadMultipleFiles(c *gin.Context)
	GetDocument(c *gin.Context)
	DownloadFile(c *gin.Context)
	ListDocuments(c *gin.Context)
	ListDocumentsByEntity(c *gin.Context)
	UpdateDocument(c *gin.Context)
	DeleteDocument(c *gin.Context)
	// GenerateShareLink(c *gin.Context)
	GeneratePresignedUploadURL(c *gin.Context)
	// ConfirmUpload(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) Handler {
	return &handler{svc: svc}
}

// UploadFile godoc
// @Summary Upload a file
// @Description Upload a single file with optional entity association
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Security BearerToken
// @Param file formData file true "File to upload"
// @Param entity_type formData string false "Entity type (practitioner, clinic, transaction, etc.)"
// @Param entity_id formData string false "Entity ID (UUID)"
// @Param is_public formData boolean false "Make file publicly accessible"
// @Param description formData string false "File description"
// @Success 201 {object} response.RsBase{data=RsUploadDocument}
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 413 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /files/upload [post]
func (h *handler) UploadFile(c *gin.Context) {
	// Get user info from context
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

	// Parse form data
	var req RqUploadFile
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Get uploaded file
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

	// Upload file
	result, err := h.svc.UploadFile(c.Request.Context(), file, fileHeader, &req, userID, *rolePtr)
	if err != nil {
		if errors.Is(err, upload.ErrFileTooLarge) {
			response.Error(c, http.StatusRequestEntityTooLarge, err)
			return
		}
		if errors.Is(err, upload.ErrInvalidFileType) {
			response.Error(c, http.StatusUnsupportedMediaType, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, result, "File uploaded successfully")
}

// UploadMultipleFiles godoc
// @Summary Upload multiple files
// @Description Upload multiple files with optional entity association
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Security BearerToken
// @Param files formData file true "Files to upload" multiple
// @Param entity_type formData string false "Entity type"
// @Param entity_id formData string false "Entity ID (UUID)"
// @Param is_public formData boolean false "Make files publicly accessible"
// @Success 201 {object} response.RsBase{data=[]RsUploadDocument}
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /files/upload/multiple [post]
func (h *handler) UploadMultipleFiles(c *gin.Context) {
	// Get user info from context
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

	// Parse form data
	var req RqUploadFile
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Get uploaded files
	form, err := c.MultipartForm()
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("failed to parse multipart form"))
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		response.Error(c, http.StatusBadRequest, errors.New("at least one file is required"))
		return
	}

	// Upload files
	results, err := h.svc.UploadMultipleFiles(c.Request.Context(), files, &req, userID, *rolePtr)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, results, "Files uploaded successfully")
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

// DownloadFile godoc
// @Summary Download a file
// @Description Download a file by ID
// @Tags files
// @Produce application/octet-stream
// @Security BearerToken
// @Param id path string true "Document ID (UUID)"
// @Param token query string false "Temporary access token"
// @Param inline query boolean false "Display inline in browser"
// @Success 200 {file} binary
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Router /files/{id}/download [get]
func (h *handler) DownloadFile(c *gin.Context) {
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

	// Download file
	file, doc, err := h.svc.DownloadFile(c.Request.Context(), docID, userID)
	if err != nil {
		if errors.Is(err, ErrDocumentNotFound) || errors.Is(err, upload.ErrFileNotFound) {
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
	defer file.Close()

	// Set headers
	c.Header("Content-Type", doc.MimeType)
	c.Header("Content-Length", strconv.FormatInt(doc.SizeBytes, 10))

	// Check if inline display is requested
	inline := c.Query("inline") == "true"
	if inline {
		c.Header("Content-Disposition", "inline; filename=\""+doc.OriginalName+"\"")
	} else {
		c.Header("Content-Disposition", "attachment; filename=\""+doc.OriginalName+"\"")
	}

	// Stream file
	if _, err := io.Copy(c.Writer, file); err != nil {
		// Can't send error response after streaming starts
		c.Abort()
		return
	}
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

// ListDocumentsByEntity godoc
// @Summary List documents by entity
// @Description List documents associated with a specific entity
// @Tags files
// @Produce json
// @Security BearerToken
// @Param entity_type path string true "Entity type"
// @Param entity_id path string true "Entity ID (UUID)"
// @Param status query string false "Filter by status"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} response.RsBase{data=RsListDocuments}
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Router /files/entity/{entity_type}/{entity_id} [get]
func (h *handler) ListDocumentsByEntity(c *gin.Context) {
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

	// Get entity info
	entityType := c.Param("entity_type")
	entityID, err := uuid.Parse(c.Param("entity_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid entity id"))
		return
	}

	// Parse query parameters
	var filters RqListDocuments
	if err := c.ShouldBindQuery(&filters); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// List documents
	result, err := h.svc.ListDocumentsByEntity(c.Request.Context(), entityType, entityID, userID, &filters)
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

// GenerateShareLink godoc
// @Summary Generate temporary share link
// @Description Generate a temporary link for sharing a document
// @Tags files
// @Accept json
// @Produce json
// @Security BearerToken
// @Param id path string true "Document ID (UUID)"
// @Param request body RqGenerateShareLink true "Share link parameters"
// @Success 200 {object} response.RsBase{data=RsShareLink}
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Router /files/{id}/share [post]
func (h *handler) GenerateShareLink(c *gin.Context) {
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
	var req RqGenerateShareLink
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Generate share link
	link, err := h.svc.GenerateShareLink(c.Request.Context(), docID, userID, req.ExpiresIn)
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

	response.JSON(c, http.StatusOK, link, "Share link generated successfully")
}

// GeneratePresignedUploadURL godoc
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
func (h *handler) GeneratePresignedUploadURL(c *gin.Context) {
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

	// Parse request body
	var req RqGeneratePresignedUploadURL
	if err := c.ShouldBindWith(&req, binding.FormMultipart); err != nil {
		response.Error(c, http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))
		return
	}

	// Validate the request
	if err := util.ValidateStruct(&req); err != nil {
		response.Error(c, http.StatusBadRequest, fmt.Errorf("validation failed: %w", err))
		return
	}

	// Generate presigned URL
	result, err := h.svc.GeneratePresignedUploadURL(c.Request.Context(), &req, userID, *rolePtr)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Presigned upload URL generated successfully")
}

// ConfirmUpload godoc
// @Summary Confirm file upload
// @Description Confirm that a file was successfully uploaded via presigned URL
// @Tags files
// @Produce json
// @Security BearerToken
// @Param id path string true "Document ID (UUID)"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Router /files/{id}/confirm [post]
func (h *handler) ConfirmUpload(c *gin.Context) {
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

	// Confirm upload
	if err := h.svc.ConfirmUpload(c.Request.Context(), docID, userID); err != nil {
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

	response.JSON(c, http.StatusOK, nil, "Upload confirmed successfully")
}
