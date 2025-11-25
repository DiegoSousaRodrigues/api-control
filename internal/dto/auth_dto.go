package dto

import "github.com/api-control/internal/domain"

type LoginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string      `json:"token"`
	User  UserSummary `json:"user"`
}

type UserSummary struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Login string `json:"login"`
}

func ParseUserToSummary(user domain.User) UserSummary {
	return UserSummary{
		ID:    user.ID,
		Name:  user.Name,
		Login: user.Login,
	}
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,login"`
	Password string `json:"password" binding:"required,min=6"`
}
