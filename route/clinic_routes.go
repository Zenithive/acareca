package route

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/contact"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/invoice"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterClinicRoutes(v1 *gin.RouterGroup, cfg *config.Config, ContactSvc contact.IService, InvoiceSvc invoice.IService) {
	clinicV1 := v1.Group("/clinic")

	// Apply authentication and role middleware for clinic routes
	authMiddleware := middleware.Auth(cfg)
	roleMiddleware := middleware.RequireRole(util.RoleClinic)

	contactHandler := contact.NewHandler(ContactSvc)
	contact.RegisterRoutes(clinicV1, contactHandler, authMiddleware, roleMiddleware)

	invoiceHandler := invoice.NewHandler(InvoiceSvc)
	invoice.RegisterRoutes(clinicV1, invoiceHandler, authMiddleware, roleMiddleware)
}
