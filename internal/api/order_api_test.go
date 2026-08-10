package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/service"
	"github.com/gin-gonic/gin"
)

type orderServiceFake struct {
	addErr error
}

func (fake *orderServiceFake) List(int16, int16) (*[]dto.OrderResponseDTO, error) {
	return nil, nil
}
func (fake *orderServiceFake) Add(dto.OrderRequestDTO) error { return fake.addErr }
func (fake *orderServiceFake) OpenBalance(int64, int16, int16) (*dto.OpenBalanceDTO, error) {
	return nil, nil
}
func (fake *orderServiceFake) FindByID(string) (*dto.OrderResponseDTO, error) { return nil, nil }
func (fake *orderServiceFake) Update(string, dto.OrderRequestDTO) error       { return nil }

func TestOrderAddDoesNotExposeUnexpectedInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalService := service.OrderService
	service.OrderService = &orderServiceFake{addErr: errors.New("sql secret detail")}
	t.Cleanup(func() { service.OrderService = originalService })

	router := gin.New()
	router.POST("/order", OrderApi.Add)
	request := httptest.NewRequest(http.MethodPost, "/order", strings.NewReader(`{"clientId":1,"orderYear":2026,"orderMonth":1,"previousMonthPayment":0,"products":[{"productId":1,"quantity":1}]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "sql secret detail") {
		t.Fatalf("response leaked internal error: %s", response.Body.String())
	}
}

func TestOrderUpdateRejectsInvalidIDBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalService := service.OrderService
	service.OrderService = &orderServiceFake{}
	t.Cleanup(func() { service.OrderService = originalService })

	router := gin.New()
	router.PUT("/order/:id", OrderApi.Update)
	request := httptest.NewRequest(http.MethodPut, "/order/invalid", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
	}
}
