package routes

import (
	"github.com/autorei/api-control/internal/api"
	"github.com/gin-gonic/gin"
	"log"
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

	groupClient := router.Group("client")
	groupClient.GET("/list", api.ClientApi.List)
	groupClient.GET("/:id", api.ClientApi.FindByID)
	groupClient.POST("/", api.ClientApi.Add)
	groupClient.PUT("/:id", api.ClientApi.Update)
	groupClient.POST("/status/:id/:status", api.ClientApi.ChangeStatus)

	return router
}
