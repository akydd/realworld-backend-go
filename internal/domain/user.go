package domain

import (
	"context"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
)

type userRepo interface {
	InsertUser(ctx context.Context, u *RegisterUser) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, string, error)
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

func (c *UserController) LoginUser(ctx context.Context, u *LoginUser) (*User, error) {
	if u.Email == "" {
		return nil, NewValidationError("email", blankFieldErrMsg)
	}
	if u.Password == "" {
		return nil, NewValidationError("password", blankFieldErrMsg)
	}

	user, hashedPassword, err := c.repo.GetUserByEmail(ctx, u.Email)
	if err != nil {
		return nil, err
	}

	match, err := argon2id.ComparePasswordAndHash(u.Password, hashedPassword)
	if err != nil {
		return nil, err
	}
	if !match {
		return nil, &CredentialsError{}
	}

	token, err := generateToken(user.Username, c.jwtSecret)
	if err != nil {
		return nil, err
	}
	user.Token = token

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
