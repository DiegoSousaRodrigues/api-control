package api

import (
	"net/http"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/service"
	"github.com/gin-gonic/gin"
)

var AuthApi IAuthApi = &authApi{}

type IAuthApi interface {
	Login(ctx *gin.Context)
	Register(ctx *gin.Context)
}

type authApi struct{}

func (a *authApi) Login(ctx *gin.Context) {
	loginReq := &dto.LoginRequest{}

	err := ctx.ShouldBindJSON(loginReq)
	if err != nil {
		ctx.AbortWithStatusJSON(400, gin.H{"error": "Invalid request"})
		return
	}

	response, err := service.AuthService.Login(*loginReq)
	if err != nil {
		ctx.AbortWithStatusJSON(401, gin.H{"error": "Invalid credentials"})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

func (a *authApi) Register(ctx *gin.Context) {
	registerReq := &dto.RegisterRequest{}

	err := ctx.ShouldBindJSON(registerReq)
	if err != nil {
		ctx.AbortWithStatusJSON(400, gin.H{"error": err.Error()})
		return
	}

	err = service.AuthService.Register(*registerReq)
	if err != nil {
		ctx.AbortWithStatusJSON(500, gin.H{"error": "Failed to create user"})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
}
