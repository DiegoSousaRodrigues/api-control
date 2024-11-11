package api

import (
	"fmt"
	"github.com/autorei/api-control/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

var ClientApi IClientApi = &clientApi{}

type IClientApi interface {
	List(ctx *gin.Context)
}

type clientApi struct{}

func (c *clientApi)List(ctx *gin.Context) {
	clients, err := service.ClientService.List()
	if err != nil {
		fmt.Println("ERROR ON LIST CLIENT API: ", err)
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
	}
	ctx.JSON(http.StatusOK, clients)
}