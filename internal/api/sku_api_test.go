package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/service"
	"github.com/gin-gonic/gin"
)

type skuServiceFake struct {
	addCalls int
}

func (f *skuServiceFake) List() (*[]dto.SkuDTO, error)         { return nil, nil }
func (f *skuServiceFake) Add(dto.SkuUpload) error              { f.addCalls++; return nil }
func (f *skuServiceFake) ChangeStatus(string, string) error    { return nil }
func (f *skuServiceFake) FindByID(string) (*dto.SkuDTO, error) { return nil, nil }
func (f *skuServiceFake) Update(string, dto.SkuUpload) error   { return nil }

func TestSkuAddRejectsOversizedMultipartBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := &skuServiceFake{}
	originalService := service.SkuService
	service.SkuService = fake
	t.Cleanup(func() { service.SkuService = originalService })

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("product", `{"name":"Produto","purchasePrice":5.00,"salePrice":10.00}`); err != nil {
		t.Fatalf("WriteField product returned error: %v", err)
	}

	part, err := writer.CreateFormFile("file", "large.png")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := part.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}); err != nil {
		t.Fatalf("Write png header returned error: %v", err)
	}
	if _, err := part.Write([]byte(strings.Repeat("a", int(service.MaxUploadRequestBytes)))); err != nil {
		t.Fatalf("Write body returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	router := gin.New()
	router.POST("/sku", SkuApi.Add)

	request := httptest.NewRequest(http.MethodPost, "/sku", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusRequestEntityTooLarge, response.Body.String())
	}

	if fake.addCalls != 0 {
		t.Fatalf("SkuService.Add called %d times, want 0", fake.addCalls)
	}
}

func TestSkuAddAcceptsTypedProductJSONPart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &skuServiceFake{}
	originalService := service.SkuService
	service.SkuService = fake
	t.Cleanup(func() { service.SkuService = originalService })

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writeProductJSONPart(t, writer, `{"name":"Produto","purchasePrice":5.00,"salePrice":10.00}`)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.POST("/sku", SkuApi.Add)
	request := httptest.NewRequest(http.MethodPost, "/sku", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || fake.addCalls != 1 {
		t.Fatalf("status = %d calls = %d body = %s", response.Code, fake.addCalls, response.Body.String())
	}
}

func TestSkuAddRejectsStringAndExponentMoney(t *testing.T) {
	for _, product := range []string{
		`{"name":"Produto","purchasePrice":"5.00","salePrice":10.00}`,
		`{"name":"Produto","purchasePrice":5.00,"salePrice":1e2}`,
	} {
		t.Run(product, func(t *testing.T) {
			fake := &skuServiceFake{}
			originalService := service.SkuService
			service.SkuService = fake
			t.Cleanup(func() { service.SkuService = originalService })

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			writeProductJSONPart(t, writer, product)
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.POST("/sku", SkuApi.Add)
			request := httptest.NewRequest(http.MethodPost, "/sku", body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusUnprocessableEntity || fake.addCalls != 0 {
				t.Fatalf("status = %d calls = %d body = %s", response.Code, fake.addCalls, response.Body.String())
			}
		})
	}
}

func writeProductJSONPart(t *testing.T, writer *multipart.Writer, product string) {
	t.Helper()
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="product"; filename="blob"`)
	header.Set("Content-Type", "application/json")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(product)); err != nil {
		t.Fatal(err)
	}
}
