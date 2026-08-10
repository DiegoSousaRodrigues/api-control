package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/repository"
	"github.com/api-control/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var OrderApi IOrderApi = &orderApi{}

type IOrderApi interface {
	List(ctx *gin.Context)
	FindByID(ctx *gin.Context)
	Add(ctx *gin.Context)
	Update(ctx *gin.Context)
	OpenBalance(ctx *gin.Context)
}

type orderApi struct{}

func (c *orderApi) List(ctx *gin.Context) {
	year, month, err := parseCompetenceQuery(ctx)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Competencia invalida"})
		return
	}
	orderList, err := service.OrderService.List(year, month)
	if err != nil {
		fmt.Println("ERROR ON LIST ORDER API: ", err)
		status := http.StatusInternalServerError
		message := "Erro interno ao listar pedidos"
		if errors.Is(err, service.ErrOrderFutureCompetence) || errors.Is(err, dto.ErrOrderYearInvalid) || errors.Is(err, dto.ErrOrderMonthInvalid) {
			status = http.StatusUnprocessableEntity
			message = err.Error()
		}
		ctx.AbortWithStatusJSON(status, gin.H{"erro": message})
		return
	}
	ctx.JSON(http.StatusOK, orderList)
}

func (c *orderApi) Add(ctx *gin.Context) {
	orderDto := &dto.OrderRequestDTO{}

	err := ctx.ShouldBindJSON(orderDto)
	if err != nil {
		fmt.Println("ERROR ON BIND ORDER API: ", err.Error())
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Pedido invalido"})
		return
	}

	err = service.OrderService.Add(*orderDto)
	if err != nil {
		fmt.Println("ERROR ON ADD ORDER API: ", err)
		abortOrderError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{})
}

func (c *orderApi) OpenBalance(ctx *gin.Context) {
	clientID, err := strconv.ParseInt(ctx.Query("clientId"), 10, 64)
	if err != nil || clientID <= 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Cliente invalido"})
		return
	}
	year, month, err := parseCompetenceQuery(ctx)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Competencia invalida"})
		return
	}
	response, err := service.OrderService.OpenBalance(clientID, year, month)
	if err != nil {
		abortOrderError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, response)
}

func parseCompetenceQuery(ctx *gin.Context) (int16, int16, error) {
	year, err := strconv.ParseInt(ctx.Query("year"), 10, 16)
	if err != nil {
		return 0, 0, err
	}
	month, err := strconv.ParseInt(ctx.Query("month"), 10, 16)
	if err != nil {
		return 0, 0, err
	}
	return int16(year), int16(month), nil
}

func (c *orderApi) FindByID(ctx *gin.Context) {
	orderID := ctx.Param("id")
	if orderID == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "ID do pedido é obrigatório"})
		return
	}

	response, err := service.OrderService.FindByID(orderID)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": "Pedido não encontrado"})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *orderApi) Update(ctx *gin.Context) {
	orderID := ctx.Param("id")
	if orderID == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "ID do pedido é obrigatório"})
		return
	}
	parsedID, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil || parsedID <= 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "ID do pedido invalido"})
		return
	}

	err = service.OrderService.Update(orderID, dto.OrderRequestDTO{})
	if err != nil {
		fmt.Println("ERROR ON UPDATE ORDER API: ", err.Error())
		status := http.StatusInternalServerError
		if errors.Is(err, repository.ErrOrderFinancialUpdateUnsupported) {
			status = http.StatusConflict
		}
		ctx.AbortWithStatusJSON(status, gin.H{"erro": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
}

func abortOrderError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrOrderPaymentExceedsBalance),
		errors.Is(err, repository.ErrOrderCompetenceExists),
		errors.Is(err, repository.ErrOrderRetroactiveCompetence),
		errors.Is(err, repository.ErrOrderFinancialUpdateUnsupported):
		ctx.AbortWithStatusJSON(http.StatusConflict, gin.H{"erro": err.Error()})
	case errors.Is(err, gorm.ErrRecordNotFound):
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": "Cliente ou produto nao encontrado"})
	case errors.Is(err, repository.ErrOrderSkuPurchasePriceMissing):
		ctx.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"erro": repository.ErrOrderSkuPurchasePriceMissing.Error()})
	case errors.Is(err, repository.ErrOrderSkuSnapshotOutOfRange):
		ctx.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"erro": repository.ErrOrderSkuSnapshotOutOfRange.Error()})
	case errors.Is(err, repository.ErrOrderClientInactive), errors.Is(err, repository.ErrOrderSkuInactive),
		errors.Is(err, repository.ErrOrderPaymentNegative), errors.Is(err, service.ErrOrderFutureCompetence),
		errors.Is(err, repository.ErrOrderCompetenceRequired),
		errors.Is(err, dto.ErrOrderRequiresProduct), errors.Is(err, dto.ErrOrderProductQuantityPositive),
		errors.Is(err, dto.ErrOrderProductDuplicated), errors.Is(err, dto.ErrOrderClientRequired),
		errors.Is(err, dto.ErrOrderYearInvalid), errors.Is(err, dto.ErrOrderMonthInvalid),
		errors.Is(err, dto.ErrOrderPaymentRequired), errors.Is(err, dto.ErrOrderPaymentNegative):
		ctx.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"erro": err.Error()})
	default:
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"erro": "Erro interno ao processar pedido"})
	}
}
