package routes

import (
	"log"

	"github.com/api-control/internal/api"
	"github.com/api-control/internal/utils"
	"github.com/gin-gonic/gin"
)

var Server = &routes{}

type routes struct {
}

func (r *routes) Run(port string) {
	engineServer := r.setupRouter()
	if err := engineServer.Run(":" + port); err != nil {
		log.Fatalln(err)
	}
}

func (r *routes) setupRouter() *gin.Engine {
	router := gin.Default()

	groupAuth := router.Group("auth")
	{
		groupAuth.POST("/login", api.AuthApi.Login)
		// groupAuth.POST("/register", api.AuthApi.Register)
	}

	groupClient := router.Group("client", utils.JWTAuthMiddleware())
	{
		groupClient.GET("/list", api.ClientApi.List)
		groupClient.GET("/:id", api.ClientApi.FindByID)
		groupClient.POST("/", api.ClientApi.Add)
		groupClient.PUT("/:id", api.ClientApi.Update)
		groupClient.POST("/status/:id/:status", api.ClientApi.ChangeStatus)
	}

	groupSku := router.Group("sku", utils.JWTAuthMiddleware())
	{
		groupSku.GET("/list", api.SkuApi.List)
		groupSku.GET("/:id", api.SkuApi.FindByID)
		groupSku.POST("/", api.SkuApi.Add)
		groupSku.PUT("/:id", api.SkuApi.Update)
		groupSku.POST("/status/:id/:status", api.SkuApi.ChangeStatus)
	}

	groupOrder := router.Group("order", utils.JWTAuthMiddleware())
	{
		groupOrder.GET("/list", api.OrderApi.List)
		groupOrder.GET("/:id", api.OrderApi.FindByID)
		groupOrder.POST("/", api.OrderApi.Add)
		groupOrder.PUT("/:id", api.OrderApi.Update)
		groupOrder.POST("/status/:id/:status", api.OrderApi.ChangeStatus)
	}

	return router
}
