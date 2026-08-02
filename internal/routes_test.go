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
