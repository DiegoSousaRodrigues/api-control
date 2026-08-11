package reportv2

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ReportService interface {
	ClientBalance(context.Context, int64) (*ClientBalanceResponse, error)
}

type API struct{ service ReportService }

func NewAPI(service ReportService) (*API, error) {
	if service == nil {
		return nil, errors.New("report v2 service is required")
	}
	return &API{service: service}, nil
}

func (api *API) ClientBalance(ctx *gin.Context) {
	clientID, err := strconv.ParseInt(ctx.Query("clientId"), 10, 64)
	if err != nil || clientID <= 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Cliente invalido"})
		return
	}
	response, err := api.service.ClientBalance(ctx.Request.Context(), clientID)
	if err == nil {
		ctx.JSON(http.StatusOK, response)
		return
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": "Cliente nao encontrado"})
	default:
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"erro": "Erro interno ao gerar relatorio"})
	}
}
