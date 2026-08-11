package stagingv2

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// API is a staging-only handler set. Phase 1 does not register it in routes.go.
type API struct {
	service StagingService
}

type StagingService interface {
	Login(context.Context, LoginRequest) (*LoginResponse, error)
	CreateClient(context.Context, ClientRequest) (*ClientResponse, error)
	ListClients(context.Context) ([]ClientResponse, error)
	FindClient(context.Context, int64) (*ClientResponse, error)
	UpdateClient(context.Context, int64, ClientRequest) error
	SetClientActive(context.Context, int64, bool) error
	CreateProduct(context.Context, ProductRequest) (*ProductResponse, error)
	ListProducts(context.Context) ([]ProductResponse, error)
	FindProduct(context.Context, int64) (*ProductResponse, error)
	UpdateProduct(context.Context, int64, ProductRequest) error
	SetProductActive(context.Context, int64, bool) error
}

func NewAPI(service StagingService) (*API, error) {
	if service == nil {
		return nil, errors.New("staging v2 service is required")
	}
	return &API{service: service}, nil
}

func (api *API) Login(ctx *gin.Context) {
	var request LoginRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Credenciais invalidas"})
		return
	}
	response, err := api.service.Login(ctx.Request.Context(), request)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"erro": "Credenciais invalidas"})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"erro": "Erro interno"})
		return
	}
	ctx.JSON(http.StatusOK, response)
}

func (api *API) CreateClient(ctx *gin.Context) {
	var request ClientRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Cliente invalido"})
		return
	}
	response, err := api.service.CreateClient(ctx.Request.Context(), request)
	respondMutation(ctx, response, err)
}

func (api *API) ListClients(ctx *gin.Context) {
	response, err := api.service.ListClients(ctx.Request.Context())
	respondQuery(ctx, response, err)
}

func (api *API) FindClient(ctx *gin.Context) {
	id, ok := positiveID(ctx)
	if !ok {
		return
	}
	response, err := api.service.FindClient(ctx.Request.Context(), id)
	respondQuery(ctx, response, err)
}

func (api *API) UpdateClient(ctx *gin.Context) {
	id, ok := positiveID(ctx)
	if !ok {
		return
	}
	var request ClientRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Cliente invalido"})
		return
	}
	respondEmpty(ctx, api.service.UpdateClient(ctx.Request.Context(), id, request))
}

func (api *API) SetClientActive(ctx *gin.Context) {
	id, ok := positiveID(ctx)
	if !ok {
		return
	}
	var request StatusRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || request.Active == nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Status invalido"})
		return
	}
	respondEmpty(ctx, api.service.SetClientActive(ctx.Request.Context(), id, *request.Active))
}

func (api *API) CreateProduct(ctx *gin.Context) {
	var request ProductRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Produto invalido"})
		return
	}
	response, err := api.service.CreateProduct(ctx.Request.Context(), request)
	respondMutation(ctx, response, err)
}

func (api *API) ListProducts(ctx *gin.Context) {
	response, err := api.service.ListProducts(ctx.Request.Context())
	respondQuery(ctx, response, err)
}

func (api *API) FindProduct(ctx *gin.Context) {
	id, ok := positiveID(ctx)
	if !ok {
		return
	}
	response, err := api.service.FindProduct(ctx.Request.Context(), id)
	respondQuery(ctx, response, err)
}

func (api *API) UpdateProduct(ctx *gin.Context) {
	id, ok := positiveID(ctx)
	if !ok {
		return
	}
	var request ProductRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Produto invalido"})
		return
	}
	respondEmpty(ctx, api.service.UpdateProduct(ctx.Request.Context(), id, request))
}

func (api *API) SetProductActive(ctx *gin.Context) {
	id, ok := positiveID(ctx)
	if !ok {
		return
	}
	var request StatusRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || request.Active == nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "Status invalido"})
		return
	}
	respondEmpty(ctx, api.service.SetProductActive(ctx.Request.Context(), id, *request.Active))
}

func positiveID(ctx *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"erro": "ID invalido"})
		return 0, false
	}
	return id, true
}

func respondMutation(ctx *gin.Context, value any, err error) {
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, value)
}

func respondQuery(ctx *gin.Context, value any, err error) {
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, value)
}

func respondEmpty(ctx *gin.Context, err error) {
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{})
}

func respondError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"erro": "Recurso nao encontrado"})
	case errors.Is(err, ErrInvalidClient), errors.Is(err, ErrInvalidProduct), errors.Is(err, ErrInvalidIdentifier):
		ctx.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"erro": "Dados invalidos"})
	default:
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"erro": "Erro interno"})
	}
}
