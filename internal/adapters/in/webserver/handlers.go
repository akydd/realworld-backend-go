package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"realworld-backend-go/internal/domain"
	"strings"
)

type userService interface {
	RegisterUser(ctx context.Context, u *domain.RegisterUser) (*domain.User, error)
	LoginUser(ctx context.Context, u *domain.LoginUser) (*domain.User, error)
	GetUser(ctx context.Context, token string) (*domain.User, error)
	UpdateUser(ctx context.Context, token string, u *domain.UpdateUser) (*domain.User, error)
}

type Handler struct {
	service userService
}

func NewHandler(s userService) *Handler {
	return &Handler{
		service: s,
	}
}

type LoginUserInner struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginUserRequest struct {
	User LoginUserInner `json:"user"`
}

type RegisterUserInner struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterUserRequest struct {
	User RegisterUserInner `json:"user"`
}

// NullableString distinguishes a JSON field being absent (Present=false)
// from being explicitly set to null or "" (Present=true, Value=nil/"").
type NullableString struct {
	Value   *string
	Present bool
}

func (n *NullableString) UnmarshalJSON(data []byte) error {
	n.Present = true
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	n.Value = &s
	return nil
}

type UpdateUserInner struct {
	Email    *string        `json:"email"`
	Bio      NullableString `json:"bio"`
	Image    NullableString `json:"image"`
	Username *string        `json:"username"`
	Password *string        `json:"password"`
}

type UpdateUserRequest struct {
	User UpdateUserInner `json:"user"`
}

type UserResponseInner struct {
	Email    string  `json:"email"`
	Token    string  `json:"token"`
	Username string  `json:"username"`
	Bio      *string `json:"bio"`
	Image    *string `json:"image"`
}

type UserResponse struct {
	User UserResponseInner `json:"user"`
}

type ErrorResponse struct {
	Errors map[string][]string `json:"errors"`
}

func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var regUser RegisterUserRequest
	err := json.NewDecoder(r.Body).Decode(&regUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d := domain.RegisterUser(regUser.User)

	w.Header().Set("Content-Type", "application/json")

	user, err := h.service.RegisterUser(r.Context(), &d)
	if err != nil {
		var errResp []byte
		var validationErr *domain.ValidationError
		var dupErr *domain.DuplicateError
		if errors.As(err, &validationErr) {
			errResp = createErrResponse(validationErr.Field, validationErr.Errors)
			w.WriteHeader(http.StatusUnprocessableEntity)
		} else if errors.As(err, &dupErr) {
			errResp = createErrResponse(dupErr.Field, []string{dupErr.Msg})
			w.WriteHeader(http.StatusConflict)
		} else {
			fmt.Println(err.Error())
			errResp = createErrResponse("unknown_error", []string{err.Error()})
			w.WriteHeader(http.StatusInternalServerError)
		}

		_, _ = w.Write(errResp)
		return
	}

	resp := UserResponse{
		User: UserResponseInner(*user),
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req LoginUserRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d := domain.LoginUser(req.User)

	w.Header().Set("Content-Type", "application/json")

	user, err := h.service.LoginUser(r.Context(), &d)
	if err != nil {
		var errResp []byte
		var validationErr *domain.ValidationError
		var credErr *domain.CredentialsError
		if errors.As(err, &validationErr) {
			errResp = createErrResponse(validationErr.Field, validationErr.Errors)
			w.WriteHeader(http.StatusUnprocessableEntity)
		} else if errors.As(err, &credErr) {
			errResp = createErrResponse("credentials", []string{"invalid"})
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			fmt.Println(err.Error())
			errResp = createErrResponse("unknown_error", []string{err.Error()})
			w.WriteHeader(http.StatusInternalServerError)
		}
		_, _ = w.Write(errResp)
		return
	}

	resp := UserResponse{
		User: UserResponseInner(*user),
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	w.Header().Set("Content-Type", "application/json")

	const prefix = "Token "
	if authHeader == "" || !strings.HasPrefix(authHeader, prefix) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(createErrResponse("token", []string{"is missing"}))
		return
	}

	rawToken := strings.TrimPrefix(authHeader, prefix)

	user, err := h.service.GetUser(r.Context(), rawToken)
	if err != nil {
		var credErr *domain.CredentialsError
		if errors.As(err, &credErr) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	resp := UserResponse{
		User: UserResponseInner(*user),
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	w.Header().Set("Content-Type", "application/json")

	const prefix = "Token "
	if authHeader == "" || !strings.HasPrefix(authHeader, prefix) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(createErrResponse("token", []string{"is missing"}))
		return
	}

	rawToken := strings.TrimPrefix(authHeader, prefix)

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d := domain.UpdateUser{
		Email:    req.User.Email,
		Username: req.User.Username,
		Password: req.User.Password,
	}
	if req.User.Bio.Present {
		if req.User.Bio.Value != nil && *req.User.Bio.Value != "" {
			d.Bio = &req.User.Bio.Value
		} else {
			d.Bio = new(*string) // pointer to nil *string = set to null
		}
	}
	if req.User.Image.Present {
		if req.User.Image.Value != nil && *req.User.Image.Value != "" {
			d.Image = &req.User.Image.Value
		} else {
			d.Image = new(*string)
		}
	}

	user, err := h.service.UpdateUser(r.Context(), rawToken, &d)
	if err != nil {
		var validationErr *domain.ValidationError
		var credErr *domain.CredentialsError
		var dupErr *domain.DuplicateError
		if errors.As(err, &validationErr) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write(createErrResponse(validationErr.Field, validationErr.Errors))
		} else if errors.As(err, &credErr) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
		} else if errors.As(err, &dupErr) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write(createErrResponse(dupErr.Field, []string{dupErr.Msg}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	resp := UserResponse{
		User: UserResponseInner(*user),
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func createErrResponse(k string, v []string) []byte {
	errResp := ErrorResponse{
		Errors: map[string][]string{
			k: v,
		},
	}
	jsonErrResp, _ := json.Marshal(errResp)
	return jsonErrResp
}
