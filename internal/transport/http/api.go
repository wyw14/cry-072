package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/platform"
)

type API struct {
	Catalog       *application.CatalogService
	Rules         *application.RuleService
	Hazards       *application.HazardService
	Workflow      *application.WorkflowService
	Remediation   *application.RemediationService
	Notifications *application.NotificationService
	Analytics     *application.AnalyticsService
	Reports       *application.ReportService
	Attachments   platform.AttachmentStore
	Validate      *validator.Validate
}

type requestScope struct {
	api     *API
	context *gin.Context
}

func newRequestScope(api *API, context *gin.Context) requestScope {
	return requestScope{api: api, context: context}
}

func decodePayload[T any](request requestScope, validate bool) (T, bool) {
	var input T
	if err := request.context.ShouldBindJSON(&input); err != nil {
		writeError(request.context, err)
		return input, false
	}
	if validate {
		if err := request.api.Validate.Struct(input); err != nil {
			writeError(request.context, err)
			return input, false
		}
	}
	return input, true
}

func (r requestScope) version() (int64, bool) {
	value, err := expectedVersion(r.context)
	if err != nil {
		writeError(r.context, err)
		return 0, false
	}
	return value, true
}

func (r requestScope) reply(status int, value any, err error) {
	if err != nil {
		writeError(r.context, err)
		return
	}
	r.context.JSON(status, value)
}

func NewAPI(catalog *application.CatalogService, rules *application.RuleService, hazards *application.HazardService, workflow *application.WorkflowService, remediation *application.RemediationService, notifications *application.NotificationService, analytics *application.AnalyticsService, reports *application.ReportService, attachments platform.AttachmentStore) *API {
	return &API{
		Catalog: catalog, Rules: rules, Hazards: hazards, Workflow: workflow,
		Remediation: remediation, Notifications: notifications, Analytics: analytics,
		Reports: reports, Attachments: attachments, Validate: validator.New(validator.WithRequiredStructEnabled()),
	}
}
