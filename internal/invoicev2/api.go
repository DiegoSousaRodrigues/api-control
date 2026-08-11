package invoicev2

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/api-control/internal/accountv2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxMutationBody = 64 << 10

type InvoiceService interface {
	Issue(context.Context, IssueRequest) (*MutationResponse, error)
	List(context.Context, ListFilter) (*ListResponse, error)
	Detail(context.Context, int64) (*InvoiceDetail, error)
	Cancel(context.Context, int64, CancelRequest) (*MutationResponse, error)
}

type API struct{ service InvoiceService }

func NewAPI(service InvoiceService) (*API, error) {
	if service == nil {
		return nil, errors.New("invoice v2 service is required")
	}
	return &API{service: service}, nil
}

func (api *API) Issue(ctx *gin.Context) {
	var request IssueRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Fatura invalida"})
		return
	}
	response, err := api.service.Issue(ctx.Request.Context(), request)
	respond(ctx, http.StatusCreated, response, err)
}

func (api *API) List(ctx *gin.Context) {
	year, errYear := strconv.Atoi(ctx.Query("year"))
	month, errMonth := strconv.Atoi(ctx.Query("month"))
	if errYear != nil || errMonth != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Filtros invalidos"})
		return
	}
	filter := ListFilter{Year: year, Month: month, Cursor: ctx.Query("cursor")}
	if raw := ctx.Query("clientId"); raw != "" {
		clientID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || clientID <= 0 {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Filtros invalidos"})
			return
		}
		filter.ClientID = &clientID
	}
	if raw := ctx.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Filtros invalidos"})
			return
		}
		filter.Limit = limit
	}
	response, err := api.service.List(ctx.Request.Context(), filter)
	respond(ctx, http.StatusOK, response, err)
}

func (api *API) Detail(ctx *gin.Context) {
	id, ok := invoiceID(ctx)
	if !ok {
		return
	}
	response, err := api.service.Detail(ctx.Request.Context(), id)
	respond(ctx, http.StatusOK, response, err)
}

func (api *API) Cancel(ctx *gin.Context) {
	id, ok := invoiceID(ctx)
	if !ok {
		return
	}
	var request CancelRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Cancelamento invalido"})
		return
	}
	response, err := api.service.Cancel(ctx.Request.Context(), id, request)
	respond(ctx, http.StatusOK, response, err)
}

func invoiceID(ctx *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "ID invalido"})
		return 0, false
	}
	return id, true
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

func respond(ctx *gin.Context, successStatus int, response any, err error) {
	if err == nil {
		ctx.JSON(successStatus, response)
		return
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": "Fatura nao encontrada"})
	case errors.Is(err, ErrInvalidCursor):
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Filtros invalidos"})
	case errors.Is(err, ErrInvoiceAlreadyExists), errors.Is(err, ErrLaterInvoiceExists),
		errors.Is(err, ErrInvoiceNotIssued), errors.Is(err, ErrInvoiceNotLatest):
		ctx.AbortWithStatusJSON(http.StatusConflict, gin.H{"erro": "Conflito no estado da fatura"})
	case errors.Is(err, ErrInvalidRequest), errors.Is(err, ErrFuturePeriod), errors.Is(err, ErrInactiveClient),
		errors.Is(err, ErrInactiveProduct):
		ctx.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"erro": "Dados da fatura invalidos"})
	case errors.Is(err, accountv2.ErrInvoiceOverallocated), errors.Is(err, accountv2.ErrPaymentOverallocated),
		errors.Is(err, accountv2.ErrInvalidActiveAllocation), errors.Is(err, ErrPersistedTotal):
		ctx.AbortWithStatusJSON(http.StatusConflict, gin.H{"erro": "Conflito financeiro"})
	default:
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"erro": "Erro interno"})
	}
}
