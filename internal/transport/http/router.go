package httptransport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/middleware"
	"go.uber.org/zap"
)

const operationIDKey = "operation_id"

type endpoint struct {
	method      string
	path        string
	operationID string
	handler     gin.HandlerFunc
}

func apiEndpoints(api *API) []endpoint {
	return []endpoint{
		{http.MethodPost, "/sites", "catalog.createSite", api.createSite},
		{http.MethodPost, "/facilities", "catalog.createFacility", api.createFacility},
		{http.MethodPost, "/risk-categories", "catalog.createRiskCategory", api.createRiskCategory},
		{http.MethodPost, "/escalation-rules", "rules.create", api.createRule},
		{http.MethodGet, "/escalation-rules", "rules.list", api.listRules},
		{http.MethodPut, "/escalation-rules/:id", "rules.revise", api.reviseRule},
		{http.MethodPost, "/hazards", "hazards.report", api.reportHazard},
		{http.MethodPost, "/inspections", "inspections.complete", api.createInspection},
		{http.MethodPost, "/attachments", "evidence.upload", api.uploadAttachment},
		{http.MethodGet, "/hazards/:id", "hazards.detail", api.getHazard},
		{http.MethodGet, "/hazard-queues", "queues.list", api.listQueue},
		{http.MethodPost, "/hazards/:id/downgrade", "hazards.downgrade", api.downgradeHazard},
		{http.MethodPost, "/hazards/:id/assign", "hazards.assign", api.assignHazard},
		{http.MethodPost, "/hazards/:id/transitions", "hazards.transition", api.transitionHazard},
		{http.MethodPut, "/hazards/:id/remediation-plan", "remediation.savePlan", api.saveRemediationPlan},
		{http.MethodPost, "/hazards/:id/remediation-tasks/:task_id/complete", "remediation.completeTask", api.completeRemediationTask},
		{http.MethodPost, "/hazards/:id/reinspections", "reinspection.record", api.reinspectHazard},
		{http.MethodPost, "/notifications/process-due", "notifications.processDue", api.processDueNotifications},
		{http.MethodPost, "/hazards/:id/notifications/acknowledge", "notifications.acknowledge", api.acknowledgeNotification},
		{http.MethodGet, "/analytics/queue-comparison", "analytics.compareQueues", api.queueAnalytics},
		{http.MethodGet, "/hazards/:id/audit", "audit.timeline", api.auditTimeline},
		{http.MethodGet, "/hazards/:id/audit.csv", "audit.exportCSV", api.exportAudit},
	}
}

func operation(operationID string, handler gin.HandlerFunc) gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Set(operationIDKey, operationID)
		handler(context)
	}
}

func registerEndpoints(group *gin.RouterGroup, endpoints []endpoint) {
	for _, route := range endpoints {
		group.Handle(route.method, route.path, operation(route.operationID, route.handler))
	}
}

func registerProbes(engine *gin.Engine, repository application.Repository) {
	engine.Handle(http.MethodGet, "/healthz", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	engine.Handle(http.MethodGet, "/readyz", func(context *gin.Context) {
		if err := repository.CheckReady(context.Request.Context()); err != nil {
			writeError(context, err)
			return
		}
		context.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}

func platformMiddleware(logger *zap.Logger) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.RequestID(),
		middleware.Recovery(logger),
		middleware.AccessLog(logger),
		middleware.SecurityHeaders(),
		middleware.CORS([]string{"http://localhost:5173", "http://127.0.0.1:5173"}),
	}
}

func NewRouter(api *API, repository application.Repository, logger *zap.Logger, defaultOperator string, defaultRole domain.Role) *gin.Engine {
	engine := gin.New()
	engine.Use(platformMiddleware(logger)...)
	registerProbes(engine, repository)

	v1 := engine.Group("/api/v1", middleware.Operator(defaultOperator, defaultRole))
	registerEndpoints(v1, apiEndpoints(api))
	return engine
}
