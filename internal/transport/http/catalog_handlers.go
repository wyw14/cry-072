package httptransport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wyw14/cry-072/internal/domain"
)

func (a *API) createSite(c *gin.Context) {
	request := newRequestScope(a, c)
	input, ok := decodePayload[domain.Site](request, true)
	if !ok {
		return
	}
	result, err := a.Catalog.CreateSite(c.Request.Context(), input, meta(c))
	request.reply(http.StatusCreated, result, err)
}

func (a *API) createFacility(c *gin.Context) {
	request := newRequestScope(a, c)
	input, ok := decodePayload[domain.Facility](request, false)
	if !ok {
		return
	}
	result, err := a.Catalog.CreateFacility(c.Request.Context(), input, meta(c))
	request.reply(http.StatusCreated, result, err)
}

func (a *API) createRiskCategory(c *gin.Context) {
	request := newRequestScope(a, c)
	input, ok := decodePayload[domain.RiskCategory](request, false)
	if !ok {
		return
	}
	result, err := a.Catalog.CreateRiskCategory(c.Request.Context(), input)
	request.reply(http.StatusCreated, result, err)
}
