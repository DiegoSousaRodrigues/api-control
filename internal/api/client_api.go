package api

import (
	"fmt"
	"github.com/autorei/api-control/internal/dto"
	"github.com/autorei/api-control/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

var ClientApi IClientApi = &clientApi{}

type IClientApi interface {
	List(ctx *gin.Context)
	FindByID(ctx *gin.Context)
	Add(ctx *gin.Context)
}

type clientApi struct{}

func (c *clientApi) FindByID(ctx *gin.Context) {
	clientID := ctx.Param("id")
	if clientID == "" {
		ctx.AbortWithStatusJSON(500, gin.H{"erro": "Necessario ID do cliente"})
		return
	}

	response, err := service.ClientService.FindByID(clientID)
	if err != nil {
		ctx.AbortWithStatusJSON(500, gin.H{"erro": "Necessario ID do cliente"})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *clientApi) Add(ctx *gin.Context) {
	clientDto := &dto.ClientRequest{}

	err := ctx.ShouldBind(clientDto)
	if err != nil {
		fmt.Println("ERROR ON BIND CLIENT API: ", err.Error())
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}

	err = service.ClientService.Add(*clientDto)
	if err != nil {
		fmt.Println("ERROR ON ADD CLIENT API: ", err)
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
}

func (c *clientApi) List(ctx *gin.Context) {
	clients, err := service.ClientService.List()
	if err != nil {
		fmt.Println("ERROR ON LIST CLIENT API: ", err)
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, clients)
}
