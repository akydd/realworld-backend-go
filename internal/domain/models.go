package domain

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
	Email    string
	Token    string
	Username string
	Bio      *string
	Image    *string
}
