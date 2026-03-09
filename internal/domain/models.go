package domain

import "time"

type RegisterUser struct {
	Username string
	Email    string
	Password string
}

type LoginUser struct {
	Email    string
	Password string
}

type User struct {
	ID       int
	Email    string
	Token    string
	Username string
	Bio      *string
	Image    *string
}

type UpdateUser struct {
	Email    *string
	Bio      **string // nil = not provided; non-nil: *Bio==nil means set to null, *Bio!=nil means set to value
	Image    **string // nil = not provided; non-nil: *Image==nil means set to null, *Image!=nil means set to value
	Username *string
	Password *string
}

type Profile struct {
	Username  string
	Bio       *string
	Image     *string
	Following bool
}

type UpdateUserData struct {
	Email    string
	Username string
	Password string
	Bio      *string
	Image    *string
}

type Article struct {
	Slug           string
	Title          string
	Description    string
	Body           string
	TagList        []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Favorited      bool
	FavoritesCount int
	Author         Profile
}

type CreateArticle struct {
	Title       string
	Description string
	Body        string
	TagList     []string
}
