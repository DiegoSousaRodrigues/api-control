package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/api-control/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var ReportApi IReportApi = &reportApi{}

type IReportApi interface {
	ClientBalance(*gin.Context)
}

type reportApi struct{}

func (api *reportApi) ClientBalance(ctx *gin.Context) {
	clientID, err := strconv.ParseInt(ctx.Query("clientId"), 10, 64)
	if err != nil || clientID <= 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Cliente invalido"})
		return
	}

	report, err := service.ReportService.ClientBalance(ctx.Request.Context(), clientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": "Cliente nao encontrado"})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"erro": "Erro interno ao gerar relatorio"})
		return
	}
	ctx.JSON(http.StatusOK, report)
}
