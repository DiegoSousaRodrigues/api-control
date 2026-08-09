package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/service"
	"github.com/gin-gonic/gin"
)

var SkuApi ISkuApi = &skuApi{}

type ISkuApi interface {
	List(ctx *gin.Context)
	FindByID(ctx *gin.Context)
	Add(ctx *gin.Context)
	ChangeStatus(ctx *gin.Context)
	Update(ctx *gin.Context)
}

type skuApi struct{}

func (c *skuApi) List(ctx *gin.Context) {
	skuList, err := service.SkuService.List()
	if err != nil {
		fmt.Println("ERROR ON LIST SKU API: ", err)
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, skuList)
}

func (c *skuApi) Add(ctx *gin.Context) {
	skuDto := &dto.SkuDTO{}
	limitUploadRequest(ctx)

	err := ctx.ShouldBind(skuDto)
	if err != nil {
		fmt.Println("ERROR ON BIND SKU API: ", err.Error())
		abortSkuUploadError(ctx, err)
		return
	}

	err = service.SkuService.Add(*skuDto)
	if err != nil {
		fmt.Println("ERROR ON ADD SKU API: ", err)
		abortSkuUploadError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
}

func (c *skuApi) ChangeStatus(ctx *gin.Context) {
	skuID := ctx.Param("id")
	if skuID == "" {
		ctx.AbortWithStatusJSON(500, gin.H{"erro": "Necessario ID do SKUe"})
		return
	}

	status := ctx.Param("status")
	if status == "" {
		ctx.AbortWithStatusJSON(500, gin.H{"erro": "Necessario ID do SKUe"})
		return
	}

	err := service.SkuService.ChangeStatus(skuID, status)
	if err != nil {
		fmt.Println("ERROR ON SERVICE SKU API: ", err.Error())
		ctx.AbortWithStatusJSON(500, gin.H{"erro": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
}

func (c *skuApi) FindByID(ctx *gin.Context) {
	skuID := ctx.Param("id")
	if skuID == "" {
		ctx.AbortWithStatusJSON(500, gin.H{"erro": "Necessario ID do SKUe"})
		return
	}

	response, err := service.SkuService.FindByID(skuID)
	if err != nil {
		ctx.AbortWithStatusJSON(500, gin.H{"erro": "Necessario ID do SKUe"})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *skuApi) Update(ctx *gin.Context) {
	dtoSku := &dto.SkuDTO{}
	limitUploadRequest(ctx)

	skuID := ctx.Param("id")
	if skuID == "" {
		ctx.AbortWithStatusJSON(500, gin.H{"erro": "Necessario ID do produto"})
		return
	}

	err := ctx.ShouldBind(&dtoSku)
	if err != nil {
		fmt.Println("ERROR ON BIND SKU API: ", err.Error())
		abortSkuUploadError(ctx, err)
		return
	}

	err = service.SkuService.Update(skuID, *dtoSku)
	if err != nil {
		fmt.Println("ERROR ON SERVICE SKU API: ", err.Error())
		abortSkuUploadError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
}

func limitUploadRequest(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, service.MaxUploadRequestBytes)
}

func abortSkuUploadError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUploadFileTooLarge) || strings.Contains(err.Error(), "request body too large"):
		ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"erro": "Imagem excede o tamanho maximo permitido"})
	case errors.Is(err, service.ErrUploadUnsupportedType):
		ctx.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{"erro": "Tipo de imagem nao permitido"})
	case errors.Is(err, service.ErrUploadEmptyFile):
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Imagem vazia"})
	default:
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"erro": err.Error()})
	}
}
