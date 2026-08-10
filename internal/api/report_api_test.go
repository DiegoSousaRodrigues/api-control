package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type reportServiceFake struct {
	report *dto.ClientBalanceReportDTO
	err    error
	calls  int
}

func (fake *reportServiceFake) ClientBalance(context.Context, int64) (*dto.ClientBalanceReportDTO, error) {
	fake.calls++
	return fake.report, fake.err
}

func serveClientBalance(t *testing.T, rawQuery string, fake *reportServiceFake) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	original := service.ReportService
	service.ReportService = fake
	t.Cleanup(func() { service.ReportService = original })
	router := gin.New()
	router.GET("/report/client-balance", ReportApi.ClientBalance)
	request := httptest.NewRequest(http.MethodGet, "/report/client-balance"+rawQuery, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestClientBalanceRejectsInvalidClientBeforeService(t *testing.T) {
	for _, query := range []string{"", "?clientId=0", "?clientId=-1", "?clientId=text", "?clientId=9223372036854775808"} {
		fake := &reportServiceFake{}
		response := serveClientBalance(t, query, fake)
		if response.Code != http.StatusBadRequest || fake.calls != 0 {
			t.Fatalf("query %q: status/calls = %d/%d, want 400/0", query, response.Code, fake.calls)
		}
	}
}

func TestClientBalanceReturnsNotFound(t *testing.T) {
	response := serveClientBalance(t, "?clientId=99", &reportServiceFake{err: gorm.ErrRecordNotFound})
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestClientBalanceDoesNotLeakInternalErrors(t *testing.T) {
	response := serveClientBalance(t, "?clientId=1", &reportServiceFake{err: errors.New("secret sql detail")})
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), "secret sql detail") {
		t.Fatalf("response leaked internal error: %s", response.Body.String())
	}
}

func TestClientBalanceReturnsNumericMoneyAndNullLegacyCost(t *testing.T) {
	report := &dto.ClientBalanceReportDTO{
		Client: dto.ReportClientDTO{ID: 1, Name: "Cliente", Active: true},
		Totals: dto.ReportBalanceTotalsDTO{SaleTotal: dto.NewMoney(decimal.RequireFromString("15.00")), CostComplete: false, MissingCostItemCount: 1},
		Months: []dto.ClientBalanceMonthDTO{},
	}
	response := serveClientBalance(t, "?clientId=1", &reportServiceFake{report: report})
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, `"saleTotal":15.00`) || !strings.Contains(body, `"purchaseTotal":null`) || !strings.Contains(body, `"months":[]`) {
		t.Fatalf("unexpected JSON contract: %s", body)
	}
}
