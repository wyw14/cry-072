package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/platform"
	"github.com/wyw14/cry-072/internal/repository/memory"
	"go.uber.org/zap"
)

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	repository := memory.New()
	clock := platform.NewManualTimeSource(time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC))
	ids := &platform.SequenceIDGenerator{}
	ctx := context.Background()
	if err := application.SeedDemo(ctx, repository, clock.Now()); err != nil {
		t.Fatal(err)
	}
	attachments, err := platform.NewLocalAttachmentStore(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	notifier := platform.NewLocalNotificationAdapter()
	api := NewAPI(application.NewCatalogService(repository, clock, ids), application.NewRuleService(repository, clock, ids), application.NewHazardService(repository, clock, ids), application.NewWorkflowService(repository, clock, ids), application.NewRemediationService(repository, clock, ids), application.NewNotificationService(repository, clock, ids, notifier), application.NewAnalyticsService(repository, clock), application.NewReportService(repository), attachments)
	return NewRouter(api, repository, zap.NewNop(), "supervisor", domain.RoleSupervisor)
}

func TestReportHazardHTTPRunsPublicHandlerAndReturnsFocusQueue(t *testing.T) {
	router := testRouter(t)
	payload := application.ReportHazardInput{SiteID: "site_demo", FacilityID: "facility_demo", InspectionItemID: "item-demo", RiskCategoryID: "risk_electrical", Title: "电缆接头温升异常", Description: "现场红外测温连续超过限值", Severity: 5, Likelihood: 5, ImpactScopes: []string{"people"}, InitialAction: "隔离支路", IdempotencyKey: "http-report-1", Evidence: []application.EvidenceInput{{Kind: "image", FileName: "thermal.png", ContentType: "image/png", StorageKey: "abc.png", SizeBytes: 200}}}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/hazards", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "request-http-1")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var hazard domain.Hazard
	if err := json.Unmarshal(response.Body.Bytes(), &hazard); err != nil {
		t.Fatal(err)
	}
	if hazard.Queue != domain.QueueFocus || hazard.MatchedRuleID != "rule_critical_electrical" {
		t.Fatalf("unexpected hazard: %#v", hazard)
	}
	if response.Header().Get("X-Request-ID") != "request-http-1" {
		t.Fatalf("request id missing: %s", response.Header().Get("X-Request-ID"))
	}
}

func TestInvalidTransitionReturnsStableErrorShape(t *testing.T) {
	router := testRouter(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/hazards/missing/transitions?version=1", bytes.NewBufferString(`{"target_state":"closed"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["code"] != "NOT_FOUND" || result["request_id"] == "" {
		t.Fatalf("unexpected error response: %#v", result)
	}
}

func TestQueueEndpointRejectsUnknownBusinessFilters(t *testing.T) {
	router := testRouter(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/hazard-queues?queue=archived&risk_level=unknown", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["code"] != "VALIDATION_ERROR" {
		t.Fatalf("unexpected error response: %#v", result)
	}
}

func TestOperationalEndpointsRejectUnsafeControlWindows(t *testing.T) {
	router := testRouter(t)
	cases := []struct {
		name string
		path string
	}{
		{name: "reversed analytics window", path: "/api/v1/analytics/queue-comparison?from=2026-08-23T00:00:00Z&to=2026-08-22T00:00:00Z"},
		{name: "oversized notification batch", path: "/api/v1/notifications/process-due?limit=201"},
		{name: "invalid audit page", path: "/api/v1/hazards/missing/audit?page=0&page_size=20"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			method := http.MethodGet
			if testCase.name == "oversized notification batch" {
				method = http.MethodPost
			}
			request := httptest.NewRequest(method, testCase.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			var result map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result["code"] != "VALIDATION_ERROR" || result["request_id"] == "" {
				t.Fatalf("unexpected error response: %#v", result)
			}
		})
	}
}
