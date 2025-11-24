package repository

import (
	"github.com/api-control/internal/domain"
)

var UserRepository IUserRepository = &userRepository{}

type IUserRepository interface {
	FindByEmail(email string) (entity *domain.User, err error)
	Add(entity domain.User) (err error)
}

type userRepository struct {
	db domain.BaseRepository
}

func (u *userRepository) FindByEmail(email string) (entity *domain.User, err error) {
	db := u.db.PSQL()

	var user domain.User
	if err := db.Where("email = ? AND active = true", email).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *userRepository) Add(user domain.User) (err error) {
	db := u.db.PSQL()

	if err := db.Create(&user).Error; err != nil {
		return err
	}

	return nil
}
