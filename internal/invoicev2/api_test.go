package invoicev2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type stubInvoiceService struct {
	issue  func(context.Context, IssueRequest) (*MutationResponse, error)
	list   func(context.Context, ListFilter) (*ListResponse, error)
	detail func(context.Context, int64) (*InvoiceDetail, error)
	cancel func(context.Context, int64, CancelRequest) (*MutationResponse, error)
}

func (stub stubInvoiceService) Issue(ctx context.Context, request IssueRequest) (*MutationResponse, error) {
	return stub.issue(ctx, request)
}
func (stub stubInvoiceService) List(ctx context.Context, filter ListFilter) (*ListResponse, error) {
	return stub.list(ctx, filter)
}
func (stub stubInvoiceService) Detail(ctx context.Context, id int64) (*InvoiceDetail, error) {
	return stub.detail(ctx, id)
}
func (stub stubInvoiceService) Cancel(ctx context.Context, id int64, request CancelRequest) (*MutationResponse, error) {
	return stub.cancel(ctx, id, request)
}

func newInvoiceRouter(t *testing.T, service InvoiceService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	api, err := NewAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.POST("/invoice", api.Issue)
	router.GET("/invoice/list", api.List)
	router.GET("/invoice/:id", api.Detail)
	router.POST("/invoice/:id/cancel", api.Cancel)
	return router
}

func TestIssueRejectsUnknownFieldsBeforeService(t *testing.T) {
	called := false
	router := newInvoiceRouter(t, stubInvoiceService{issue: func(context.Context, IssueRequest) (*MutationResponse, error) {
		called = true
		return nil, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/invoice", strings.NewReader(`{
		"clientId":1,"year":2026,"month":8,"products":[{"productId":1,"quantity":1}],"priceTotal":1
	}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || called {
		t.Fatalf("status=%d called=%v body=%s", recorder.Code, called, recorder.Body.String())
	}
}

func TestIssueReturnsJSONNumbersAndCreated(t *testing.T) {
	response := &MutationResponse{Invoice: InvoiceDetail{InvoiceSummary: InvoiceSummary{
		ID: 10, ProductsTotal: newAmount(decimal.RequireFromString("500.00")),
		PaidAmount: newAmount(decimal.RequireFromString("300.00")), OpenAmount: newAmount(decimal.RequireFromString("200.00")),
	}, Items: make([]InvoiceItemResponse, 0)}, Account: AccountResponse{NetBalance: newAmount(decimal.RequireFromString("200.00"))}}
	router := newInvoiceRouter(t, stubInvoiceService{issue: func(context.Context, IssueRequest) (*MutationResponse, error) {
		return response, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/invoice", bytes.NewBufferString(`{"clientId":1,"year":2026,"month":8,"products":[{"productId":1,"quantity":1}]}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	invoice := body["invoice"].(map[string]any)
	if _, ok := invoice["productsTotal"].(float64); !ok {
		t.Fatalf("productsTotal must be a JSON number: %T", invoice["productsTotal"])
	}
}

func TestConflictAndInternalErrorsAreSanitized(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "business conflict", err: ErrInvoiceAlreadyExists, want: http.StatusConflict},
		{name: "internal", err: errors.New("postgres secret detail"), want: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newInvoiceRouter(t, stubInvoiceService{cancel: func(context.Context, int64, CancelRequest) (*MutationResponse, error) {
				return nil, test.err
			}})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/invoice/10/cancel", strings.NewReader(`{"reason":"wrong"}`))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.want || strings.Contains(recorder.Body.String(), "postgres") {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestListParsesContractFiltersAndKeepsEmptyItemsArray(t *testing.T) {
	router := newInvoiceRouter(t, stubInvoiceService{list: func(_ context.Context, filter ListFilter) (*ListResponse, error) {
		if filter.Year != 2026 || filter.Month != 8 || filter.ClientID == nil || *filter.ClientID != 42 || filter.Limit != 25 {
			t.Fatalf("filter = %#v", filter)
		}
		return &ListResponse{Items: make([]InvoiceSummary, 0)}, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/invoice/list?year=2026&month=8&clientId=42&limit=25", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"items":[]`) || !strings.Contains(recorder.Body.String(), `"nextCursor":null`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
