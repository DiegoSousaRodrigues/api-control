package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOrderStatusRouteIsNotRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := (&routes{}).setupRouter()

	for _, route := range router.Routes() {
		if route.Path == "/order/status/:id/:status" {
			t.Fatalf("unsafe order status route is still registered with method %s", route.Method)
		}
	}

	request := httptest.NewRequest(http.MethodPost, "/order/status/1/true", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("POST /order/status/1/true status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestOrderOpenBalanceRouteIsRegistered(t *testing.T) {
	router := (&routes{}).setupRouter()
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/order/open-balance" {
			return
		}
	}
	t.Fatal("GET /order/open-balance route is not registered")
}

func TestClientBalanceReportRouteIsRegisteredBehindJWT(t *testing.T) {
	router := (&routes{}).setupRouter()
	found := false
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/report/client-balance" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("GET /report/client-balance route is not registered")
	}

	request := httptest.NewRequest(http.MethodGet, "/report/client-balance?clientId=1", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated report status = %d, want 401", response.Code)
	}
}
