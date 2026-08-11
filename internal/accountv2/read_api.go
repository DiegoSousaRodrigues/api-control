package accountv2

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AccountReadService interface {
	Summary(context.Context, int64) (*AccountSummaryResponse, error)
	Statement(context.Context, int64, StatementFilter) (*StatementResponse, error)
}

type ReadAPI struct{ service AccountReadService }

func NewReadAPI(service AccountReadService) (*ReadAPI, error) {
	if service == nil {
		return nil, errors.New("account v2 read service is required")
	}
	return &ReadAPI{service: service}, nil
}

func (api *ReadAPI) Summary(ctx *gin.Context) {
	id, ok := readClientID(ctx)
	if !ok {
		return
	}
	response, err := api.service.Summary(ctx.Request.Context(), id)
	respondRead(ctx, response, err)
}

func (api *ReadAPI) Statement(ctx *gin.Context) {
	id, ok := readClientID(ctx)
	if !ok {
		return
	}
	filter := StatementFilter{Cursor: ctx.Query("cursor")}
	if raw := ctx.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			readBadRequest(ctx)
			return
		}
		filter.Limit = limit
	}
	if raw := ctx.Query("dateTo"); raw != "" {
		dateTo, err := time.Parse("2006-01-02", raw)
		if err != nil {
			readBadRequest(ctx)
			return
		}
		filter.DateTo = &dateTo
	}
	response, err := api.service.Statement(ctx.Request.Context(), id, filter)
	respondRead(ctx, response, err)
}

func readClientID(ctx *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		readBadRequest(ctx)
		return 0, false
	}
	return id, true
}

func readBadRequest(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Parametros invalidos"})
}

func respondRead(ctx *gin.Context, response any, err error) {
	if err == nil {
		ctx.JSON(http.StatusOK, response)
		return
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": "Cliente nao encontrado"})
	case errors.Is(err, ErrInvalidReadRequest), errors.Is(err, ErrInvalidStatementCursor):
		readBadRequest(ctx)
	default:
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"erro": "Erro interno"})
	}
}
