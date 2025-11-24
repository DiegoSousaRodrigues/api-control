package service

import (
	"github.com/api-control/internal/domain"
	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/repository"
	"github.com/api-control/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

var AuthService IAuthService = &authService{}

type IAuthService interface {
	Login(loginReq dto.LoginRequest) (*dto.LoginResponse, error)
	Register(registerReq dto.RegisterRequest) error
}

type authService struct{}

func (a *authService) Login(loginReq dto.LoginRequest) (*dto.LoginResponse, error) {
	user, err := repository.UserRepository.FindByEmail(loginReq.Email)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginReq.Password))
	if err != nil {
		return nil, err
	}

	token, err := utils.GenerateJWT(user.ID)
	if err != nil {
		return nil, err
	}

	userSummary := dto.ParseUserToSummary(*user)
	return &dto.LoginResponse{
		Token: token,
		User:  userSummary,
	}, nil
}

func (a *authService) Register(registerReq dto.RegisterRequest) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(registerReq.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := domain.User{
		Name:     registerReq.Name,
		Email:    registerReq.Email,
		Password: string(hashedPassword),
		Active:   true,
	}

	return repository.UserRepository.Add(user)
}
