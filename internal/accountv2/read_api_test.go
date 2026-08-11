package accountv2

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type readAPIServiceStub struct {
	summary   func(context.Context, int64) (*AccountSummaryResponse, error)
	statement func(context.Context, int64, StatementFilter) (*StatementResponse, error)
}

func (stub readAPIServiceStub) Summary(ctx context.Context, id int64) (*AccountSummaryResponse, error) {
	return stub.summary(ctx, id)
}

func (stub readAPIServiceStub) Statement(ctx context.Context, id int64, filter StatementFilter) (*StatementResponse, error) {
	return stub.statement(ctx, id, filter)
}

func accountReadRouter(t *testing.T, service AccountReadService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	api, err := NewReadAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/client/:id/account", api.Summary)
	router.GET("/client/:id/account/statement", api.Statement)
	return router
}

func TestStatementAPIParsesInclusiveDateAndPagination(t *testing.T) {
	router := accountReadRouter(t, readAPIServiceStub{statement: func(_ context.Context, id int64, filter StatementFilter) (*StatementResponse, error) {
		if id != 42 || filter.Limit != 25 || filter.Cursor != "opaque" || filter.DateTo == nil ||
			filter.DateTo.Format("2006-01-02") != "2026-08-31" {
			t.Fatalf("id=%d filter=%#v", id, filter)
		}
		return &StatementResponse{Items: make([]StatementItem, 0)}, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/client/42/account/statement?dateTo=2026-08-31&limit=25&cursor=opaque", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAccountReadAPIErrorsAreSanitized(t *testing.T) {
	router := accountReadRouter(t, readAPIServiceStub{summary: func(context.Context, int64) (*AccountSummaryResponse, error) {
		return nil, errors.New("SQL contained a connection secret")
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/client/1/account", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError || strings.Contains(strings.ToLower(recorder.Body.String()), "sql") ||
		strings.Contains(strings.ToLower(recorder.Body.String()), "secret") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestStatementAPIRejectsInvalidCivilDateBeforeService(t *testing.T) {
	called := false
	router := accountReadRouter(t, readAPIServiceStub{statement: func(context.Context, int64, StatementFilter) (*StatementResponse, error) {
		called = true
		return nil, nil
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/client/1/account/statement?dateTo=2026-02-30", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || called {
		t.Fatalf("status=%d called=%v", recorder.Code, called)
	}
}
