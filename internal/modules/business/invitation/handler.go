package invitation

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	SendInvitation(c *gin.Context)
	GetInvitation(c *gin.Context)
	ProcessInvitation(c *gin.Context)
	ListInvitations(c *gin.Context)
	ResendInvitation(c *gin.Context)
	RevokeInvitation(c *gin.Context)
	HandlePermissions(c *gin.Context)
	ListAccountantPermissions(c *gin.Context)
}

type Handler struct {
	svc            Service
	accountantRepo accountant.Repository
}

func NewHandler(svc Service, accountantRepo accountant.Repository) IHandler {
	return &Handler{svc: svc, accountantRepo: accountantRepo}
}

// @Summary      Send an invitation
// @Description  Practitioner sends an invitation to a person's email.
// @Tags         invitation
// @Accept       json
// @Produce      json
// @Param        request body RqSendInvitation true "Email of the Accountant/Bookkeeper"
// @Success      201 {object} response.RsBase
// @Failure      400 {object} response.RsError
// @Failure      500 {object} response.RsError
// @Security     BearerToken
// @Router       /invite/ [post]
func (h *Handler) SendInvitation(c *gin.Context) {
	practID, ok := util.GetPractitionerID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, nil)
		return
	}

	var req RqSendInvitation
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	res, err := h.svc.SendInvite(c.Request.Context(), practID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, res, "Invitation sent successfully")
}

// @Summary      Get invitation details
// @Description  Check if an invitation exists and if the user is already registered.
// @Tags         invitation
// @Produce      json
// @Param        id     path   string  true  "Invitation UUID"
// @Success      200 {object} response.RsBase
// @Failure      400 {object} response.RsError
// @Failure      500 {object} response.RsError
// @Router       /invite/{id} [get]
func (h *Handler) GetInvitation(c *gin.Context) {
	idParam := c.Param("id")
	inviteID, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	res, err := h.svc.GetInvitationDetails(c.Request.Context(), inviteID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, res, "Invitation details fetched successfully")
}

// @Summary      Process invitation action
// @Description  Accept or Reject an invitation via POST.
// @Tags         invitation
// @Accept       json
// @Produce      json
// @Param        request body RqProcessAction true "Token ID and Action (accept/reject)"
// @Success      200 {object} response.RsBase
// @Failure      400 {object} response.RsError
// @Failure      500 {object} response.RsError
// @Router       /invite/process [post]
func (h *Handler) ProcessInvitation(c *gin.Context) {
	var req RqProcessAction
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	res, err := h.svc.ProcessInvitation(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, res, "Invitation processed")
}

// @Summary      List invitations
// @Description  Retrieve a paginated list of invitations. If logged in as a Practitioner, it shows sent invites. If an Accountant, it shows received invites.
// @Tags         invitation
// @Produce      json
// @Param        email    query     string  false  "Filter by exact email address"
// @Param        status   query     string  false  "Filter by status (SENT, ACCEPTED, COMPLETED, REJECTED)"
// @Param        search   query     string  false  "Fuzzy search by email"
// @Param        limit    query     int     false  "Number of records to return (default 10, max 100)"
// @Param        offset   query     int     false  "Number of records to skip"
// @Param        sort_by  query     string  false  "Field to sort by (e.g., created_at, email)"
// @Param        order_by query     string  false  "Order of sorting (ASC or DESC)"
// @Success      200      {object}  util.RsList
// @Failure      400      {object}  response.RsError
// @Failure      401      {object}  response.RsError
// @Failure      403      {object}  response.RsError
// @Failure      500      {object}  response.RsError
// @Security     BearerToken
// @Router       /invite [get]
func (h *Handler) ListInvitations(c *gin.Context) {
	pIDPtr, aIDPtr, ok := util.GetRoleBasedIDs(c)
	if !ok {
		return
	}

	var reqFilter Filter
	if err := c.ShouldBindQuery(&reqFilter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	res, err := h.svc.ListInvitations(c.Request.Context(), pIDPtr, aIDPtr, &reqFilter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, res, "Invitations retrieved successfully")
}

// @Summary      Resend an invitation
// @Description  Invalidates the old invitation and sends a new one to the same email.
// @Tags         invitation
// @Param        id   path   string  true  "Invitation ID"
// @Success      200      {object}  util.RsList
// @Failure      400      {object}  response.RsError
// @Failure      401      {object}  response.RsError
// @Failure      500      {object}  response.RsError
// @Security     BearerToken
// @Router       /invite/{id}/resend [post]
func (h *Handler) ResendInvitation(c *gin.Context) {
	practID, ok := util.GetPractitionerID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, nil)
		return
	}

	inviteIDStr := c.Param("id")
	inviteID, err := uuid.Parse(inviteIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	res, err := h.svc.ResendInvite(c.Request.Context(), practID, inviteID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, res, "Invitation resent successfully")
}

// @Summary      Revoke an accepted/completed invitation
// @Description  Practitioner removes an accountant's access by revoking the invitation. Only works on ACCEPTED or COMPLETED invitations.
// @Tags         invitation
// @Produce      json
// @Param        id   path   string  true  "Invitation ID"
// @Success      200  {object}  response.RsBase
// @Failure      400  {object}  response.RsError
// @Failure      401  {object}  response.RsError
// @Failure      403  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /invite/{id}/revoke [delete]
func (h *Handler) RevokeInvitation(c *gin.Context) {
	practID, ok := util.GetPractitionerID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, nil)
		return
	}

	inviteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.RevokeInvite(c.Request.Context(), practID, inviteID); err != nil {
		switch {
		case errors.Is(err, ErrInvitationNotFound):
			response.Error(c, http.StatusNotFound, err)
		case errors.Is(err, ErrUnauthorizedInvite):
			response.Error(c, http.StatusForbidden, err)
		case errors.Is(err, ErrInvitationAlreadyUsed), errors.Is(err, ErrCannotRevokeStatus):
			response.Error(c, http.StatusBadRequest, err)
		default:
			response.Error(c, http.StatusInternalServerError, err)
		}
		return
	}

	response.JSON(c, http.StatusOK, nil, "Invitation revoked successfully")
}

// ListAccountantPermissions godoc
// @Summary      List Accountant Permissions
// @Description   Retrieve all active entity permissions (Clinics, Forms) assigned to the logged-in Accountant.
// @Tags         invitation
// @Produce      json
// @Success      200      {object}  util.RsList
// @Failure      400      {object}  response.RsError
// @Failure      401      {object}  response.RsError
// @Failure      500      {object}  response.RsError
// @Security     BearerToken
// @Router       /invite/list-permissions [get]
func (h *Handler) ListAccountantPermissions(c *gin.Context) {
	accId, ok := util.GetAccountantID(c)
	if !ok {
		return
	}

	var reqFilter Filter
	if err := c.ShouldBindQuery(&reqFilter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	// 2. Call Service with the Accountant ID pointer
	res, err := h.svc.ListAccountantPermissions(c.Request.Context(), accId, &reqFilter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, res, "Permissions retrieved successfully")
}

// @Summary      Grant or Update permissions
// @Description  Practitioner grants specific permissions (Read, Create, Update, Delete, All) to an accountant for a specific entity.
// @Tags         invitation
// @Accept       json
// @Produce      json
// @Param        request body RqGrantPermission true "Permission Details"
// @Success      200 {object} response.RsBase
// @Failure      400 {object} response.RsError
// @Security     BearerToken
// @Router       /invite/permissions [post]
func (h *Handler) HandlePermissions(c *gin.Context) {
	practID, ok := util.GetPractitionerID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, nil)
		return
	}

	var req RqGrantPermission
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	var results []interface{}

	// Loop through each permission detail in the request
	for _, p := range req.Permissions {
		// Normalize EntityType
		p.EntityType = strings.ToUpper(p.EntityType)

		targetAID := uuid.Nil
		if req.AccountantID != nil {
			targetAID = *req.AccountantID
		}

		resPerms, err := h.svc.GrantEntityPermission(
			c.Request.Context(),
			practID,
			targetAID,
			p.EntityID,
			req.Email,
			p.EntityType,
			p.Permissions,
		)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		results = append(results, gin.H{
			"entity_id":   p.EntityID,
			"entity_type": p.EntityType,
			"permissions": resPerms,
		})
	}

	data := gin.H{
		"accountant_id": req.AccountantID,
		"results":       results,
	}

	response.JSON(c, http.StatusOK, data, "Permissions Processed Successfully")
}
