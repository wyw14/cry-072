package domain

import (
	"strings"
	"time"
)

type Site struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Region    string    `json:"region"`
	State     SiteState `json:"state"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s Site) Validate() error {
	if strings.TrimSpace(s.Code) == "" {
		return NewValidationError("code", "场地编码不能为空")
	}
	if strings.TrimSpace(s.Name) == "" {
		return NewValidationError("name", "场地名称不能为空")
	}
	if s.State != SiteActive && s.State != SiteIsolated {
		return NewValidationError("state", "场地状态无效")
	}
	return nil
}

type Facility struct {
	ID        string    `json:"id"`
	SiteID    string    `json:"site_id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Critical  bool      `json:"critical"`
	Enabled   bool      `json:"enabled"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (f Facility) Validate() error {
	if strings.TrimSpace(f.SiteID) == "" {
		return NewValidationError("site_id", "设施必须属于场地")
	}
	if strings.TrimSpace(f.Code) == "" || strings.TrimSpace(f.Name) == "" {
		return NewValidationError("facility", "设施编码和名称不能为空")
	}
	return nil
}

type InspectionItem struct {
	ID             string    `json:"id"`
	FacilityKind   string    `json:"facility_kind"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	RiskCategoryID string    `json:"risk_category_id"`
	Required       bool      `json:"required"`
	Enabled        bool      `json:"enabled"`
	Version        int64     `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
}

type RiskCategory struct {
	ID          string    `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	BaseWeight  int       `json:"base_weight"`
	DefaultSLA  int       `json:"default_sla_minutes"`
	Description string    `json:"description"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
}

func (r RiskCategory) Validate() error {
	if strings.TrimSpace(r.Code) == "" || strings.TrimSpace(r.Name) == "" {
		return NewValidationError("risk_category", "风险分类编码和名称不能为空")
	}
	if r.BaseWeight < 0 || r.BaseWeight > 100 {
		return NewValidationError("base_weight", "基础权重必须在 0 到 100 之间")
	}
	if r.DefaultSLA <= 0 {
		return NewValidationError("default_sla_minutes", "处置时限必须为正数")
	}
	return nil
}
