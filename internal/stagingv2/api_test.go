package stagingv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type stagingServiceFake struct{}

func (stagingServiceFake) Login(context.Context, LoginRequest) (*LoginResponse, error) {
	return nil, nil
}
func (stagingServiceFake) CreateClient(context.Context, ClientRequest) (*ClientResponse, error) {
	return nil, nil
}
func (stagingServiceFake) ListClients(context.Context) ([]ClientResponse, error) {
	return []ClientResponse{}, nil
}
func (stagingServiceFake) FindClient(context.Context, int64) (*ClientResponse, error) {
	return nil, nil
}
func (stagingServiceFake) UpdateClient(context.Context, int64, ClientRequest) error { return nil }
func (stagingServiceFake) SetClientActive(context.Context, int64, bool) error       { return nil }
func (stagingServiceFake) CreateProduct(context.Context, ProductRequest) (*ProductResponse, error) {
	return nil, nil
}
func (stagingServiceFake) ListProducts(context.Context) ([]ProductResponse, error) {
	return []ProductResponse{}, nil
}
func (stagingServiceFake) FindProduct(context.Context, int64) (*ProductResponse, error) {
	return nil, nil
}
func (stagingServiceFake) UpdateProduct(context.Context, int64, ProductRequest) error { return nil }
func (stagingServiceFake) SetProductActive(context.Context, int64, bool) error        { return nil }

func TestStagingHandlerRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api, err := NewAPI(stagingServiceFake{})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/clients/:id", api.FindClient)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/clients/invalid", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestStagingHandlersAreNotActivatedInLegacyRoutes(t *testing.T) {
	routes, err := os.ReadFile("../routes.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(routes), "stagingv2") || strings.Contains(string(routes), "schema_v2") {
		t.Fatal("staging v2 was activated in legacy routes")
	}
}
