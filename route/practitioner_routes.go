package route

import (
	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/pkg/config"
)

func RegisterPractitionerRoutes(v1 *gin.RouterGroup, cfg *config.Config, practitionerSvc practitioner.IService) {
	practitionerHandler := practitioner.NewHandler(practitionerSvc)
	practitioner.RegisterRoutes(v1, practitionerHandler, cfg)
}
