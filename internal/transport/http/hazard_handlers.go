package httptransport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
)

type queueQuery struct {
	Queue      domain.QueueKind   `form:"queue"`
	OwnerID    string             `form:"owner_id"`
	State      domain.HazardState `form:"state"`
	RiskLevel  domain.RiskLevel   `form:"risk_level"`
	SiteID     string             `form:"site_id"`
	Overdue    bool               `form:"overdue"`
	Page       int                `form:"page"`
	PageSize   int                `form:"page_size"`
	Sort       string             `form:"sort"`
	Descending bool               `form:"descending"`
}

func (q queueQuery) filter() (domain.QueueFilter, error) {
	if q.Queue != "" && !q.Queue.Valid() {
		return domain.QueueFilter{}, domain.NewValidationError("queue", "队列只允许 normal 或 focus")
	}
	if q.State != "" && !q.State.Valid() {
		return domain.QueueFilter{}, domain.NewValidationError("state", "隐患状态不在允许范围内")
	}
	if q.RiskLevel != "" && !q.RiskLevel.Valid() {
		return domain.QueueFilter{}, domain.NewValidationError("risk_level", "风险等级不在允许范围内")
	}
	return domain.QueueFilter{
		Queue: q.Queue, OwnerID: q.OwnerID, State: q.State, RiskLevel: q.RiskLevel, SiteID: q.SiteID,
		OverdueOnly: q.Overdue, Page: q.Page, PageSize: q.PageSize, Sort: q.Sort, Descending: q.Descending,
	}.Normalize(), nil
}

type downgradeCommand struct {
	Reason string `json:"reason" validate:"required,min=10,max=1000"`
}

type assignmentCommand struct {
	OwnerID string `json:"owner_id" validate:"required,max=120"`
}

func (a *API) createInspection(c *gin.Context) {
	request := newRequestScope(a, c)
	input, ok := decodePayload[domain.Inspection](request, false)
	if !ok {
		return
	}
	result, err := a.Hazards.CreateInspection(c.Request.Context(), input, meta(c))
	request.reply(http.StatusCreated, result, err)
}

func (a *API) uploadAttachment(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		writeError(c, domain.NewValidationError("file", "必须上传一个证据文件"))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		writeError(c, err)
		return
	}
	defer file.Close()
	contentType := fileHeader.Header.Get("Content-Type")
	stored, err := a.Attachments.Save(c.Request.Context(), fileHeader.Filename, contentType, file)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"storage_key": stored.Key, "size_bytes": stored.Size, "content_type": stored.ContentType})
}

func (a *API) reportHazard(c *gin.Context) {
	request := newRequestScope(a, c)
	input, ok := decodePayload[application.ReportHazardInput](request, true)
	if !ok {
		return
	}
	result, err := a.Hazards.Report(c.Request.Context(), input, meta(c))
	request.reply(http.StatusCreated, result, err)
}

func (a *API) getHazard(c *gin.Context) {
	request := newRequestScope(a, c)
	result, err := a.Hazards.Get(c.Request.Context(), c.Param("id"))
	request.reply(http.StatusOK, result, err)
}

func (a *API) listQueue(c *gin.Context) {
	request := newRequestScope(a, c)
	query := queueQuery{Page: 1, PageSize: 20, Sort: "due_at"}
	if err := c.ShouldBindQuery(&query); err != nil {
		writeError(c, domain.NewValidationError("query", "队列筛选参数格式不正确"))
		return
	}
	filter, err := query.filter()
	if err != nil {
		writeError(c, err)
		return
	}
	result, err := a.Hazards.ListQueue(c.Request.Context(), filter)
	request.reply(http.StatusOK, result, err)
}

func (a *API) downgradeHazard(c *gin.Context) {
	request := newRequestScope(a, c)
	version, ok := request.version()
	if !ok {
		return
	}
	input, ok := decodePayload[downgradeCommand](request, true)
	if !ok {
		return
	}
	result, err := a.Hazards.ManualDowngrade(c.Request.Context(), c.Param("id"), input.Reason, version, meta(c))
	request.reply(http.StatusOK, result, err)
}

func (a *API) assignHazard(c *gin.Context) {
	request := newRequestScope(a, c)
	version, ok := request.version()
	if !ok {
		return
	}
	input, ok := decodePayload[assignmentCommand](request, true)
	if !ok {
		return
	}
	result, err := a.Hazards.AssignOwner(c.Request.Context(), c.Param("id"), input.OwnerID, version, meta(c))
	request.reply(http.StatusOK, result, err)
}

func (a *API) transitionHazard(c *gin.Context) {
	request := newRequestScope(a, c)
	version, ok := request.version()
	if !ok {
		return
	}
	input, ok := decodePayload[domain.TransitionRequest](request, false)
	if !ok {
		return
	}
	result, err := a.Workflow.Transition(c.Request.Context(), c.Param("id"), version, input, meta(c))
	request.reply(http.StatusOK, result, err)
}
