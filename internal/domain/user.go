package domain

import (
	"context"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
)

type userRepo interface {
	InsertUser(ctx context.Context, u *RegisterUser) (*User, error)
}

type UserController struct {
	repo      userRepo
	jwtSecret string
}

func New(r userRepo, jwtSecret string) *UserController {
	return &UserController{
		repo:      r,
		jwtSecret: jwtSecret,
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

	token, err := generateToken(user.Username, c.jwtSecret)
	if err != nil {
		return nil, err
	}
	user.Token = token

	return user, nil
}

func generateToken(username string, secret string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   username,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
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
