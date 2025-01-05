package api

import (
	"fmt"
	"net/http"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/service"
	"github.com/gin-gonic/gin"
)

var SkuApi ISkuApi = &skuApi{}

type ISkuApi interface {
	List(ctx *gin.Context)
	Add(ctx *gin.Context)
}

type skuApi struct{}

func (c *skuApi) List(ctx *gin.Context) {
	clients, err := service.SkuService.List()
	if err != nil {
		fmt.Println("ERROR ON LIST CLIENT API: ", err)
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, clients)
}

func (c *skuApi) Add(ctx *gin.Context) {
	skuDto := &dto.SkuDTO{}

	err := ctx.ShouldBind(skuDto)
	if err != nil {
		fmt.Println("ERROR ON BIND CLIENT API: ", err.Error())
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}

	err = service.SkuService.Add(*skuDto)
	if err != nil {
		fmt.Println("ERROR ON ADD CLIENT API: ", err)
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
}