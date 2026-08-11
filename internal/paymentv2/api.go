package paymentv2

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/api-control/internal/accountv2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxMutationBody = 64 << 10

type PaymentService interface {
	Create(context.Context, CreateRequest) (*MutationResponse, error)
	List(context.Context, ListFilter) (*ListResponse, error)
	Detail(context.Context, int64) (*PaymentResponse, error)
	Reverse(context.Context, int64, ReverseRequest) (*MutationResponse, error)
}

type API struct{ service PaymentService }

func NewAPI(service PaymentService) (*API, error) {
	if service == nil {
		return nil, errors.New("payment v2 service is required")
	}
	return &API{service: service}, nil
}

func (api *API) Create(ctx *gin.Context) {
	var request CreateRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Pagamento invalido"})
		return
	}
	response, err := api.service.Create(ctx.Request.Context(), request)
	respond(ctx, http.StatusCreated, response, err)
}

func (api *API) List(ctx *gin.Context) {
	filter := ListFilter{Status: ctx.Query("status"), Cursor: ctx.Query("cursor")}
	if raw := ctx.Query("clientId"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			badFilter(ctx)
			return
		}
		filter.ClientID = &id
	}
	var ok bool
	if filter.DateFrom, ok = queryDate(ctx, "dateFrom"); !ok {
		return
	}
	if filter.DateTo, ok = queryDate(ctx, "dateTo"); !ok {
		return
	}
	if raw := ctx.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			badFilter(ctx)
			return
		}
		filter.Limit = limit
	}
	response, err := api.service.List(ctx.Request.Context(), filter)
	respond(ctx, http.StatusOK, response, err)
}

func (api *API) Detail(ctx *gin.Context) {
	id, ok := paymentID(ctx)
	if !ok {
		return
	}
	response, err := api.service.Detail(ctx.Request.Context(), id)
	respond(ctx, http.StatusOK, response, err)
}

func (api *API) Reverse(ctx *gin.Context) {
	id, ok := paymentID(ctx)
	if !ok {
		return
	}
	var request ReverseRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Estorno invalido"})
		return
	}
	response, err := api.service.Reverse(ctx.Request.Context(), id, request)
	respond(ctx, http.StatusOK, response, err)
}

func paymentID(ctx *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "ID invalido"})
		return 0, false
	}
	return id, true
}

func queryDate(ctx *gin.Context, name string) (*time.Time, bool) {
	raw := ctx.Query(name)
	if raw == "" {
		return nil, true
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		badFilter(ctx)
		return nil, false
	}
	return &value, true
}

func badFilter(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Filtros invalidos"})
}

func decodeStrictJSON(ctx *gin.Context, destination any) error {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxMutationBody)
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

func respond(ctx *gin.Context, status int, response any, err error) {
	if err == nil {
		ctx.JSON(status, response)
		return
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": "Pagamento nao encontrado"})
	case errors.Is(err, ErrInvalidCursor), errors.Is(err, ErrInvalidFilter):
		badFilter(ctx)
	case errors.Is(err, ErrPaymentNotPosted):
		ctx.AbortWithStatusJSON(http.StatusConflict, gin.H{"erro": "Conflito no estado do pagamento"})
	case errors.Is(err, ErrInvalidRequest), errors.Is(err, ErrFutureEffectiveDate):
		ctx.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"erro": "Dados do pagamento invalidos"})
	case errors.Is(err, accountv2.ErrInvoiceOverallocated), errors.Is(err, accountv2.ErrPaymentOverallocated),
		errors.Is(err, accountv2.ErrInvalidActiveAllocation):
		ctx.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"erro": "Violacao financeira"})
	default:
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"erro": "Erro interno"})
	}
}
