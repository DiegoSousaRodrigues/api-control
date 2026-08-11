package paymentv2

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type apiServiceStub struct {
	create  func(context.Context, CreateRequest) (*MutationResponse, error)
	list    func(context.Context, ListFilter) (*ListResponse, error)
	detail  func(context.Context, int64) (*PaymentResponse, error)
	reverse func(context.Context, int64, ReverseRequest) (*MutationResponse, error)
}

func (stub apiServiceStub) Create(ctx context.Context, request CreateRequest) (*MutationResponse, error) {
	return stub.create(ctx, request)
}
func (stub apiServiceStub) List(ctx context.Context, filter ListFilter) (*ListResponse, error) {
	return stub.list(ctx, filter)
}
func (stub apiServiceStub) Detail(ctx context.Context, id int64) (*PaymentResponse, error) {
	return stub.detail(ctx, id)
}
func (stub apiServiceStub) Reverse(ctx context.Context, id int64, request ReverseRequest) (*MutationResponse, error) {
	return stub.reverse(ctx, id, request)
}

func paymentRouter(t *testing.T, service PaymentService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	api, err := NewAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.POST("/payment", api.Create)
	router.GET("/payment/list", api.List)
	router.GET("/payment/:id", api.Detail)
	router.POST("/payment/:id/reverse", api.Reverse)
	return router
}

func TestCreateRejectsStringExponentAndUnknownFields(t *testing.T) {
	for _, body := range []string{
		`{"clientId":1,"amount":"10.00","effectiveDate":"2026-08-01"}`,
		`{"clientId":1,"amount":1e2,"effectiveDate":"2026-08-01"}`,
		`{"clientId":1,"amount":10,"effectiveDate":"2026-08-01","method":"cash"}`,
	} {
		called := false
		router := paymentRouter(t, apiServiceStub{create: func(context.Context, CreateRequest) (*MutationResponse, error) {
			called = true
			return nil, nil
		}})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/payment", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest || called {
			t.Fatalf("body=%s status=%d called=%v", body, recorder.Code, called)
		}
	}
}

func TestListParsesInclusiveISOFilters(t *testing.T) {
	router := paymentRouter(t, apiServiceStub{list: func(_ context.Context, filter ListFilter) (*ListResponse, error) {
		if filter.ClientID == nil || *filter.ClientID != 42 || filter.Status != "posted" || filter.Limit != 25 ||
			filter.DateFrom == nil || filter.DateFrom.Format("2006-01-02") != "2026-01-01" ||
			filter.DateTo == nil || filter.DateTo.Format("2006-01-02") != "2026-12-31" {
			t.Fatalf("filter = %#v", filter)
		}
		return &ListResponse{Items: make([]PaymentResponse, 0)}, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/payment/list?clientId=42&dateFrom=2026-01-01&dateTo=2026-12-31&status=posted&limit=25", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestErrorsAreSanitized(t *testing.T) {
	router := paymentRouter(t, apiServiceStub{reverse: func(context.Context, int64, ReverseRequest) (*MutationResponse, error) {
		return nil, errors.New("database connection secret")
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/payment/1/reverse", strings.NewReader(`{"reason":"duplicate"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "database") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
