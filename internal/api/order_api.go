package api

import (
	"fmt"
	"net/http"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/service"
	"github.com/gin-gonic/gin"
)

var OrderApi IOrderApi = &orderApi{}

type IOrderApi interface {
	List(ctx *gin.Context)
	FindByID(ctx *gin.Context)
	Add(ctx *gin.Context)
	ChangeStatus(ctx *gin.Context)
	Update(ctx *gin.Context)
}

type orderApi struct{}

func (c *orderApi) List(ctx *gin.Context) {
	orderList, err := service.OrderService.List()
	if err != nil {
		fmt.Println("ERROR ON LIST ORDER API: ", err)
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, orderList)
}

func (c *orderApi) Add(ctx *gin.Context) {
	orderDto := &dto.OrderRequestDTO{}

	err := ctx.ShouldBind(orderDto)
	if err != nil {
		fmt.Println("ERROR ON BIND ORDER API: ", err.Error())
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}

	err = service.OrderService.Add(*orderDto)
	if err != nil {
		fmt.Println("ERROR ON ADD ORDER API: ", err)
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{})
}

func (c *orderApi) ChangeStatus(ctx *gin.Context) {
	orderID := ctx.Param("id")
	if orderID == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "ID do pedido é obrigatório"})
		return
	}

	status := ctx.Param("status")
	if status == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Status é obrigatório"})
		return
	}

	err := service.OrderService.ChangeStatus(orderID, status)
	if err != nil {
		fmt.Println("ERROR ON CHANGE STATUS ORDER API: ", err.Error())
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
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
	orderDto := &dto.OrderRequestDTO{}

	orderID := ctx.Param("id")
	if orderID == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "ID do pedido é obrigatório"})
		return
	}

	err := ctx.ShouldBind(&orderDto)
	if err != nil {
		fmt.Println("ERROR ON BIND ORDER API: ", err.Error())
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}

	err = service.OrderService.Update(orderID, *orderDto)
	if err != nil {
		fmt.Println("ERROR ON UPDATE ORDER API: ", err.Error())
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
}