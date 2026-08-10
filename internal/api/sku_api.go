package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
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

const maxSkuProductJSONBytes int64 = 64 << 10

var (
	errSkuProductPartRequired = errors.New("product part is required")
	errSkuProductMediaType    = errors.New("product part must be application/json")
	errSkuProductPartTooLarge = errors.New("product part is too large")
)

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
	limitUploadRequest(ctx)

	upload, err := parseSkuUpload(ctx)
	if err != nil {
		fmt.Println("ERROR ON BIND SKU API: ", err.Error())
		abortSkuUploadError(ctx, err)
		return
	}

	err = service.SkuService.Add(*upload)
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
	limitUploadRequest(ctx)

	skuID := ctx.Param("id")
	if skuID == "" {
		ctx.AbortWithStatusJSON(500, gin.H{"erro": "Necessario ID do produto"})
		return
	}

	upload, err := parseSkuUpload(ctx)
	if err != nil {
		fmt.Println("ERROR ON BIND SKU API: ", err.Error())
		abortSkuUploadError(ctx, err)
		return
	}

	err = service.SkuService.Update(skuID, *upload)
	if err != nil {
		fmt.Println("ERROR ON SERVICE SKU API: ", err.Error())
		abortSkuUploadError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
}

func parseSkuUpload(ctx *gin.Context) (*dto.SkuUpload, error) {
	if err := ctx.Request.ParseMultipartForm(service.MaxUploadRequestBytes); err != nil {
		return nil, err
	}

	productHeader, err := ctx.FormFile("product")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, errSkuProductPartRequired
		}
		return nil, err
	}
	mediaType, _, err := mime.ParseMediaType(productHeader.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, errSkuProductMediaType
	}
	if productHeader.Size > maxSkuProductJSONBytes {
		return nil, errSkuProductPartTooLarge
	}
	productFile, err := productHeader.Open()
	if err != nil {
		return nil, err
	}
	defer productFile.Close()

	decoder := json.NewDecoder(io.LimitReader(productFile, maxSkuProductJSONBytes+1))
	decoder.DisallowUnknownFields()
	var product dto.SkuProductRequest
	if err := decoder.Decode(&product); err != nil {
		return nil, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	if err := product.Validate(); err != nil {
		return nil, err
	}

	file, err := ctx.FormFile("file")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		return nil, err
	}
	return &dto.SkuUpload{Product: product, File: file}, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing interface{}
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("product must contain exactly one JSON object")
	}
	return err
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
	case errors.Is(err, errSkuProductMediaType):
		ctx.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{"erro": err.Error()})
	case errors.Is(err, errSkuProductPartTooLarge):
		ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"erro": err.Error()})
	case errors.Is(err, dto.ErrMoneyMustBeNumber), errors.Is(err, dto.ErrMoneyTooManyDecimals),
		errors.Is(err, dto.ErrMoneyOutOfRange), errors.Is(err, dto.ErrSkuNameRequired),
		errors.Is(err, dto.ErrSkuPurchasePriceRequired), errors.Is(err, dto.ErrSkuSalePriceRequired),
		errors.Is(err, dto.ErrSkuPurchasePriceNegative), errors.Is(err, dto.ErrSkuSalePriceNotPositive):
		ctx.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"erro": err.Error()})
	default:
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Requisicao de produto invalida"})
	}
}
