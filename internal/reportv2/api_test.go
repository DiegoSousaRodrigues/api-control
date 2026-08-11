package reportv2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type serviceStub struct {
	response *ClientBalanceResponse
	err      error
	called   bool
}

func (stub *serviceStub) ClientBalance(context.Context, int64) (*ClientBalanceResponse, error) {
	stub.called = true
	return stub.response, stub.err
}

func serveReport(t *testing.T, query string, service ReportService) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	api, err := NewAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/report/client-balance", api.ClientBalance)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/report/client-balance"+query, nil))
	return recorder
}

func TestAPIRejectsInvalidClientBeforeService(t *testing.T) {
	for _, query := range []string{"", "?clientId=abc", "?clientId=0"} {
		stub := &serviceStub{}
		response := serveReport(t, query, stub)
		if response.Code != http.StatusBadRequest || stub.called {
			t.Fatalf("query=%q status=%d called=%v", query, response.Code, stub.called)
		}
	}
}

func TestAPIReturnsExactNumericContract(t *testing.T) {
	stub := &serviceStub{response: &ClientBalanceResponse{Client: ClientResponse{ID: 1, Name: "Client", Active: true},
		Totals: TotalsResponse{InvoiceCount: 1, QuantityTotal: 2, PurchaseTotal: newAmount(decimal.NewFromInt(60)),
			SaleTotal: newAmount(decimal.NewFromInt(100)), ProfitTotal: newAmount(decimal.NewFromInt(40))},
		Months: []MonthResponse{{Year: 2026, Month: 8, TotalsResponse: TotalsResponse{InvoiceCount: 1, QuantityTotal: 2,
			PurchaseTotal: newAmount(decimal.NewFromInt(60)), SaleTotal: newAmount(decimal.NewFromInt(100)), ProfitTotal: newAmount(decimal.NewFromInt(40))}}}}}
	response := serveReport(t, "?clientId=1", stub)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	totals := body["totals"].(map[string]any)
	if _, ok := totals["saleTotal"].(float64); !ok || totals["invoiceCount"].(float64) != 1 {
		t.Fatalf("totals = %#v", totals)
	}
	for _, legacy := range []string{"orderCount", "costComplete", "missingCostItemCount"} {
		if strings.Contains(response.Body.String(), legacy) {
			t.Fatalf("response contains legacy field %q", legacy)
		}
	}
}

func TestAPISanitizesNotFoundAndInternalErrors(t *testing.T) {
	notFound := serveReport(t, "?clientId=1", &serviceStub{err: gorm.ErrRecordNotFound})
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("not found status=%d", notFound.Code)
	}
	internal := serveReport(t, "?clientId=1", &serviceStub{err: errors.New("secret sql")})
	if internal.Code != http.StatusInternalServerError || strings.Contains(internal.Body.String(), "secret") {
		t.Fatalf("internal status=%d body=%s", internal.Code, internal.Body.String())
	}
}
