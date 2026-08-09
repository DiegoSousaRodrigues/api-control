package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/service"
	"github.com/gin-gonic/gin"
)

type clientServiceFake struct {
	updateIDs []string
}

func (f *clientServiceFake) List() (*[]dto.ClientDTO, error)         { return nil, nil }
func (f *clientServiceFake) Add(dto.ClientRequest) error             { return nil }
func (f *clientServiceFake) FindByID(string) (*dto.ClientDTO, error) { return nil, nil }
func (f *clientServiceFake) ChangeStatus(string, string) error       { return nil }
func (f *clientServiceFake) Update(id string, _ dto.ClientDTO) error {
	f.updateIDs = append(f.updateIDs, id)
	return nil
}

func TestClientUpdateValidatesAndForwardsID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &clientServiceFake{}
	originalService := service.ClientService
	service.ClientService = fake
	t.Cleanup(func() { service.ClientService = originalService })

	router := gin.New()
	router.PUT("/client/:id", ClientApi.Update)

	invalidIDs := []string{"abc", "0", "-1", "9223372036854775808"}
	for _, id := range invalidIDs {
		request := httptest.NewRequest(http.MethodPut, "/client/"+id, bytes.NewBufferString(`{}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest {
			t.Errorf("PUT /client/%s status = %d, want %d", id, response.Code, http.StatusBadRequest)
		}
	}

	if len(fake.updateIDs) != 0 {
		t.Fatalf("service Update called for invalid IDs: %v", fake.updateIDs)
	}

	for _, id := range []string{"1", "2"} {
		request := httptest.NewRequest(http.MethodPut, "/client/"+id, bytes.NewBufferString(`{}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Errorf("PUT /client/%s status = %d, want %d", id, response.Code, http.StatusOK)
		}
	}

	if len(fake.updateIDs) != 2 || fake.updateIDs[0] != "1" || fake.updateIDs[1] != "2" {
		t.Fatalf("service Update IDs = %v, want [1 2]", fake.updateIDs)
	}
}
