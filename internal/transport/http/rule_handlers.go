package httptransport

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wyw14/cry-072/internal/domain"
)

type rulePayload struct {
	Name                 string               `json:"name" validate:"required,max=120"`
	Description          string               `json:"description" validate:"required,max=1000"`
	Priority             int                  `json:"priority" validate:"min=1,max=1000"`
	Enabled              bool                 `json:"enabled"`
	Condition            domain.RuleCondition `json:"condition"`
	FocusSLAMinutes      int                  `json:"focus_sla_minutes" validate:"min=1,max=43200"`
	NotifyAfterMinutes   int                  `json:"notify_after_minutes" validate:"min=0,max=43200"`
	ReNotifyEveryMinutes int                  `json:"renotify_every_minutes" validate:"min=1,max=43200"`
	Reason               string               `json:"reason"`
}

func (p rulePayload) domain() domain.EscalationRule {
	return domain.EscalationRule{
		Name: strings.TrimSpace(p.Name), Description: strings.TrimSpace(p.Description), Priority: p.Priority,
		Enabled: p.Enabled, Condition: p.Condition,
		FocusSLA:      time.Duration(p.FocusSLAMinutes) * time.Minute,
		NotifyAfter:   time.Duration(p.NotifyAfterMinutes) * time.Minute,
		ReNotifyEvery: time.Duration(p.ReNotifyEveryMinutes) * time.Minute,
	}
}

func (a *API) createRule(c *gin.Context) {
	request := newRequestScope(a, c)
	input, ok := decodePayload[rulePayload](request, true)
	if !ok {
		return
	}
	created, err := a.Rules.Create(c.Request.Context(), input.domain(), meta(c))
	request.reply(http.StatusCreated, created, err)
}

func (a *API) listRules(c *gin.Context) {
	request := newRequestScope(a, c)
	rules, err := a.Rules.ListActive(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	request.reply(http.StatusOK, gin.H{"items": rules, "total": len(rules)}, nil)
}

func (a *API) reviseRule(c *gin.Context) {
	request := newRequestScope(a, c)
	version, ok := request.version()
	if !ok {
		return
	}
	input, ok := decodePayload[rulePayload](request, true)
	if !ok {
		return
	}
	updated, err := a.Rules.Revise(c.Request.Context(), c.Param("id"), version, input.domain(), input.Reason, meta(c))
	request.reply(http.StatusOK, updated, err)
}
