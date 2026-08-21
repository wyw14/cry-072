package httptransport

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wyw14/cry-072/internal/domain"
)

const (
	defaultNotificationBatch = 50
	maxNotificationBatch     = 200
	defaultAuditPageSize     = 20
	maxAuditPageSize         = 200
)

type analyticsWindow struct {
	from time.Time
	to   time.Time
}

func readPositiveInt(c *gin.Context, name string, fallback, maximum int) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > maximum {
		return 0, domain.NewValidationError(name, "必须是允许范围内的正整数")
	}
	return value, nil
}

func readAnalyticsWindow(c *gin.Context) (analyticsWindow, error) {
	from, err := time.Parse(time.RFC3339, strings.TrimSpace(c.Query("from")))
	if err != nil {
		return analyticsWindow{}, domain.NewValidationError("from", "必须是 RFC3339 时间")
	}
	to, err := time.Parse(time.RFC3339, strings.TrimSpace(c.Query("to")))
	if err != nil {
		return analyticsWindow{}, domain.NewValidationError("to", "必须是 RFC3339 时间")
	}
	if !from.Before(to) {
		return analyticsWindow{}, domain.NewValidationError("to", "必须晚于统计开始时间")
	}
	if to.Sub(from) > 366*24*time.Hour {
		return analyticsWindow{}, domain.NewValidationError("from", "单次统计时间窗不能超过 366 天")
	}
	return analyticsWindow{from: from, to: to}, nil
}

func (a *API) processDueNotifications(c *gin.Context) {
	limit, err := readPositiveInt(c, "limit", defaultNotificationBatch, maxNotificationBatch)
	if err != nil {
		writeError(c, err)
		return
	}
	processed, err := a.Notifications.ProcessDue(c.Request.Context(), limit)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"processed": processed})
}

func (a *API) acknowledgeNotification(c *gin.Context) {
	version, err := expectedVersion(c)
	if err != nil {
		writeError(c, err)
		return
	}
	if err := a.Notifications.Acknowledge(c.Request.Context(), c.Param("id"), version, meta(c)); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) queueAnalytics(c *gin.Context) {
	window, err := readAnalyticsWindow(c)
	if err != nil {
		writeError(c, err)
		return
	}
	result, err := a.Analytics.Snapshot(c.Request.Context(), window.from, window.to)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *API) auditTimeline(c *gin.Context) {
	page, err := readPositiveInt(c, "page", 1, 1_000_000)
	if err != nil {
		writeError(c, err)
		return
	}
	pageSize, err := readPositiveInt(c, "page_size", defaultAuditPageSize, maxAuditPageSize)
	if err != nil {
		writeError(c, err)
		return
	}
	result, err := a.Hazards.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	timeline, err := a.Reports.Timeline(c.Request.Context(), result.ID, page, pageSize)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, timeline)
}

func (a *API) exportAudit(c *gin.Context) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="hazard-audit.csv"`)
	if err := a.Reports.ExportAuditCSV(c.Request.Context(), c.Param("id"), c.Writer); err != nil {
		writeError(c, err)
		return
	}
}
