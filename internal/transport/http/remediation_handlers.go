package httptransport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wyw14/cry-072/internal/domain"
)

type taskCompletionCommand struct {
	EvidenceIDs []string `json:"evidence_ids" validate:"required,min=1"`
}

func (a *API) saveRemediationPlan(c *gin.Context) {
	request := newRequestScope(a, c)
	plan, ok := decodePayload[domain.RemediationPlan](request, false)
	if !ok {
		return
	}
	plan.HazardID = c.Param("id")
	version := int64(0)
	if plan.ID != "" {
		var valid bool
		version, valid = request.version()
		if !valid {
			return
		}
	}
	status := http.StatusCreated
	if version > 0 {
		status = http.StatusOK
	}
	result, err := a.Remediation.SavePlan(c.Request.Context(), plan, version, meta(c))
	request.reply(status, result, err)
}

func (a *API) completeRemediationTask(c *gin.Context) {
	request := newRequestScope(a, c)
	input, ok := decodePayload[taskCompletionCommand](request, true)
	if !ok {
		return
	}
	version, ok := request.version()
	if !ok {
		return
	}
	result, err := a.Remediation.CompleteTask(
		c.Request.Context(), c.Param("id"), c.Param("task_id"), input.EvidenceIDs, version, meta(c),
	)
	request.reply(http.StatusOK, result, err)
}

func (a *API) reinspectHazard(c *gin.Context) {
	request := newRequestScope(a, c)
	input, ok := decodePayload[domain.Reinspection](request, false)
	if !ok {
		return
	}
	version, ok := request.version()
	if !ok {
		return
	}
	input.HazardID = c.Param("id")
	result, err := a.Remediation.Reinspect(c.Request.Context(), input, version, meta(c))
	request.reply(http.StatusCreated, result, err)
}
