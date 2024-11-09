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

	router.GET("/api/clients", api.ClientApi.List)

	return router
}
