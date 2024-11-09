package api

import (
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
	clients := service.ClientService.List()
	ctx.JSON(http.StatusOK, clients)
}