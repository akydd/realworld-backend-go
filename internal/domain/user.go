package domain

import (
	"context"
	"github.com/alexedwards/argon2id"
)

type userRepo interface {
	InsertUser(ctx context.Context, u *RegisterUser) (*User, error)
}

type UserController struct {
	repo userRepo
}

func New(r userRepo) *UserController {
	return &UserController{
		repo: r,
	}
}

func (c *UserController) RegisterUser(ctx context.Context, u *RegisterUser) (*User, error) {
	if err := validateRegisterUser(u); err != nil {
		return nil, err
	}

	hash, err := argon2id.CreateHash((u.Password), argon2id.DefaultParams)
	if err != nil {
		return nil, err
	}
	u.Password = hash

	user, err := c.repo.InsertUser(ctx, u)
	if err != nil {
		return nil, err
	}
	user.Token = "fake token"

	return user, nil
}

func validateRegisterUser(r *RegisterUser) error {
	if r.Email == "" {
		return NewValidationError("email", blankFieldErrMsg)
	}

	if r.Password == "" {
		return NewValidationError("password", blankFieldErrMsg)
	}

	if r.Username == "" {
		return NewValidationError("username", blankFieldErrMsg)
	}

	return nil
}
