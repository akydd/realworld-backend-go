package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"realworld-backend-go/internal/domain"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type userService interface {
	RegisterUser(ctx context.Context, u *domain.RegisterUser) (*domain.User, error)
	LoginUser(ctx context.Context, u *domain.LoginUser) (*domain.User, error)
	GetUser(ctx context.Context, userID int) (*domain.User, error)
	UpdateUser(ctx context.Context, userID int, u *domain.UpdateUser) (*domain.User, error)
}

type profileService interface {
	GetProfile(ctx context.Context, profileUsername string, viewerID int) (*domain.Profile, error)
	FollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error)
	UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error)
}

type articleService interface {
	CreateArticle(ctx context.Context, authorID int, a *domain.CreateArticle) (*domain.Article, error)
	GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*domain.Article, error)
	UpdateArticle(ctx context.Context, callerID int, slug string, u *domain.UpdateArticle) (*domain.Article, error)
	FavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error)
	UnfavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error)
}

type tagService interface {
	GetTags(ctx context.Context) ([]string, error)
}

type commentService interface {
	CreateComment(ctx context.Context, authorID int, articleSlug string, c *domain.CreateComment) (*domain.Comment, error)
	GetComments(ctx context.Context, articleSlug string, viewerID int) ([]*domain.Comment, error)
	DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error
}

type Handler struct {
	service        userService
	profileService profileService
	articleService articleService
	tagService     tagService
	commentService commentService
}

func NewHandler(s userService, ps profileService, as articleService, ts tagService, cs commentService) *Handler {
	return &Handler{
		service:        s,
		profileService: ps,
		articleService: as,
		tagService:     ts,
		commentService: cs,
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
		User: UserResponseInner{
			Email:    user.Email,
			Token:    user.Token,
			Username: user.Username,
			Bio:      user.Bio,
			Image:    user.Image,
		},
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
		User: UserResponseInner{
			Email:    user.Email,
			Token:    user.Token,
			Username: user.Username,
			Bio:      user.Bio,
			Image:    user.Image,
		},
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(int)

	user, err := h.service.GetUser(r.Context(), userID)
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
		User: UserResponseInner{
			Email:    user.Email,
			Token:    user.Token,
			Username: user.Username,
			Bio:      user.Bio,
			Image:    user.Image,
		},
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(int)

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

	user, err := h.service.UpdateUser(r.Context(), userID, &d)
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
		User: UserResponseInner{
			Email:    user.Email,
			Token:    user.Token,
			Username: user.Username,
			Bio:      user.Bio,
			Image:    user.Image,
		},
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

type ProfileResponseInner struct {
	Username  string  `json:"username"`
	Bio       *string `json:"bio"`
	Image     *string `json:"image"`
	Following bool    `json:"following"`
}

type ProfileResponse struct {
	Profile ProfileResponseInner `json:"profile"`
}

func profileResponse(profile *domain.Profile) ProfileResponse {
	return ProfileResponse{
		Profile: ProfileResponseInner{
			Username:  profile.Username,
			Bio:       profile.Bio,
			Image:     profile.Image,
			Following: profile.Following,
		},
	}
}

func writeArticleErr(w http.ResponseWriter, err error) {
	var notFoundErr *domain.ArticleNotFoundError
	if errors.As(err, &notFoundErr) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(createErrResponse("article", []string{"not found"}))
	} else {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
	}
}

func writeProfileErr(w http.ResponseWriter, err error) {
	var notFoundErr *domain.ProfileNotFoundError
	if errors.As(err, &notFoundErr) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(createErrResponse("profile", []string{"not found"}))
	} else {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
	}
}

func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	profileUsername := mux.Vars(r)["username"]
	viewerID, _ := r.Context().Value(userIDKey).(int)

	w.Header().Set("Content-Type", "application/json")

	profile, err := h.profileService.GetProfile(r.Context(), profileUsername, viewerID)
	if err != nil {
		writeProfileErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profileResponse(profile))
}

func (h *Handler) FollowUser(w http.ResponseWriter, r *http.Request) {
	followerID := r.Context().Value(userIDKey).(int)
	followeeUsername := mux.Vars(r)["username"]

	w.Header().Set("Content-Type", "application/json")

	profile, err := h.profileService.FollowUser(r.Context(), followerID, followeeUsername)
	if err != nil {
		writeProfileErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profileResponse(profile))
}

func (h *Handler) UnfollowUser(w http.ResponseWriter, r *http.Request) {
	followerID := r.Context().Value(userIDKey).(int)
	followeeUsername := mux.Vars(r)["username"]

	w.Header().Set("Content-Type", "application/json")

	profile, err := h.profileService.UnfollowUser(r.Context(), followerID, followeeUsername)
	if err != nil {
		writeProfileErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profileResponse(profile))
}

type CreateArticleInner struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Body        string   `json:"body"`
	TagList     []string `json:"tagList"`
}

type CreateArticleRequest struct {
	Article CreateArticleInner `json:"article"`
}

type ArticleAuthor struct {
	Username  string  `json:"username"`
	Bio       *string `json:"bio"`
	Image     *string `json:"image"`
	Following bool    `json:"following"`
}

type ArticleResponseInner struct {
	Slug           string        `json:"slug"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	Body           string        `json:"body"`
	TagList        []string      `json:"tagList"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
	Favorited      bool          `json:"favorited"`
	FavoritesCount int           `json:"favoritesCount"`
	Author         ArticleAuthor `json:"author"`
}

type ArticleResponse struct {
	Article ArticleResponseInner `json:"article"`
}

func articleResponse(a *domain.Article) ArticleResponse {
	return ArticleResponse{
		Article: ArticleResponseInner{
			Slug:           a.Slug,
			Title:          a.Title,
			Description:    a.Description,
			Body:           a.Body,
			TagList:        a.TagList,
			CreatedAt:      a.CreatedAt,
			UpdatedAt:      a.UpdatedAt,
			Favorited:      a.Favorited,
			FavoritesCount: a.FavoritesCount,
			Author: ArticleAuthor{
				Username:  a.Author.Username,
				Bio:       a.Author.Bio,
				Image:     a.Author.Image,
				Following: a.Author.Following,
			},
		},
	}
}

func (h *Handler) CreateArticle(w http.ResponseWriter, r *http.Request) {
	authorID := r.Context().Value(userIDKey).(int)

	var req CreateArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d := domain.CreateArticle{
		Title:       req.Article.Title,
		Description: req.Article.Description,
		Body:        req.Article.Body,
		TagList:     req.Article.TagList,
	}

	w.Header().Set("Content-Type", "application/json")

	article, err := h.articleService.CreateArticle(r.Context(), authorID, &d)
	if err != nil {
		var validationErr *domain.ValidationError
		var dupErr *domain.DuplicateError
		if errors.As(err, &validationErr) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write(createErrResponse(validationErr.Field, validationErr.Errors))
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

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(articleResponse(article))
}

func (h *Handler) GetArticle(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]
	viewerID, _ := r.Context().Value(userIDKey).(int)

	w.Header().Set("Content-Type", "application/json")

	article, err := h.articleService.GetArticleBySlug(r.Context(), slug, viewerID)
	if err != nil {
		writeArticleErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(articleResponse(article))
}

type UpdateArticleInner struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Body        *string `json:"body"`
}

type UpdateArticleRequest struct {
	Article UpdateArticleInner `json:"article"`
}

func (h *Handler) UpdateArticle(w http.ResponseWriter, r *http.Request) {
	callerID := r.Context().Value(userIDKey).(int)
	slug := mux.Vars(r)["slug"]

	var req UpdateArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	u := domain.UpdateArticle{
		Title:       req.Article.Title,
		Description: req.Article.Description,
		Body:        req.Article.Body,
	}

	w.Header().Set("Content-Type", "application/json")

	article, err := h.articleService.UpdateArticle(r.Context(), callerID, slug, &u)
	if err != nil {
		var validationErr *domain.ValidationError
		var notFoundErr *domain.ArticleNotFoundError
		var dupErr *domain.DuplicateError
		var credErr *domain.CredentialsError
		if errors.As(err, &validationErr) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write(createErrResponse(validationErr.Field, validationErr.Errors))
		} else if errors.As(err, &notFoundErr) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(createErrResponse("article", []string{"not found"}))
		} else if errors.As(err, &dupErr) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write(createErrResponse(dupErr.Field, []string{dupErr.Msg}))
		} else if errors.As(err, &credErr) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(articleResponse(article))
}

func (h *Handler) FavoriteArticle(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(int)
	slug := mux.Vars(r)["slug"]

	w.Header().Set("Content-Type", "application/json")

	article, err := h.articleService.FavoriteArticle(r.Context(), userID, slug)
	if err != nil {
		writeArticleErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(articleResponse(article))
}

func (h *Handler) UnfavoriteArticle(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(int)
	slug := mux.Vars(r)["slug"]

	w.Header().Set("Content-Type", "application/json")

	article, err := h.articleService.UnfavoriteArticle(r.Context(), userID, slug)
	if err != nil {
		writeArticleErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(articleResponse(article))
}

type CommentAuthor struct {
	Username  string  `json:"username"`
	Bio       *string `json:"bio"`
	Image     *string `json:"image"`
	Following bool    `json:"following"`
}

type CommentResponseInner struct {
	ID        int           `json:"id"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
	Body      string        `json:"body"`
	Author    CommentAuthor `json:"author"`
}

type CommentResponse struct {
	Comment CommentResponseInner `json:"comment"`
}

type CommentsResponse struct {
	Comments []CommentResponseInner `json:"comments"`
}

func (h *Handler) CreateArticleComment(w http.ResponseWriter, r *http.Request) {
	authorID := r.Context().Value(userIDKey).(int)
	slug := mux.Vars(r)["slug"]

	var req struct {
		Comment struct {
			Body string `json:"body"`
		} `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	comment, err := h.commentService.CreateComment(r.Context(), authorID, slug, &domain.CreateComment{Body: req.Comment.Body})
	if err != nil {
		var validationErr *domain.ValidationError
		var notFoundErr *domain.ArticleNotFoundError
		if errors.As(err, &validationErr) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write(createErrResponse(validationErr.Field, validationErr.Errors))
		} else if errors.As(err, &notFoundErr) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(createErrResponse("article", []string{"not found"}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(CommentResponse{
		Comment: CommentResponseInner{
			ID:        comment.ID,
			CreatedAt: comment.CreatedAt,
			UpdatedAt: comment.UpdatedAt,
			Body:      comment.Body,
			Author: CommentAuthor{
				Username:  comment.Author.Username,
				Bio:       comment.Author.Bio,
				Image:     comment.Author.Image,
				Following: comment.Author.Following,
			},
		},
	})
}

func (h *Handler) DeleteArticleComment(w http.ResponseWriter, r *http.Request) {
	callerID := r.Context().Value(userIDKey).(int)
	slug := mux.Vars(r)["slug"]

	commentID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(createErrResponse("id", []string{"must be an integer"}))
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := h.commentService.DeleteComment(r.Context(), callerID, slug, commentID); err != nil {
		var notFoundArticle *domain.ArticleNotFoundError
		var notFoundComment *domain.CommentNotFoundError
		var credErr *domain.CredentialsError
		if errors.As(err, &notFoundArticle) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(createErrResponse("article", []string{"not found"}))
		} else if errors.As(err, &notFoundComment) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(createErrResponse("comment", []string{"not found"}))
		} else if errors.As(err, &credErr) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetArticleComments(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]
	viewerID, _ := r.Context().Value(userIDKey).(int)

	w.Header().Set("Content-Type", "application/json")

	comments, err := h.commentService.GetComments(r.Context(), slug, viewerID)
	if err != nil {
		var notFoundErr *domain.ArticleNotFoundError
		if errors.As(err, &notFoundErr) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(createErrResponse("article", []string{"not found"}))
		} else {
			fmt.Println(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		}
		return
	}

	resp := CommentsResponse{Comments: make([]CommentResponseInner, 0, len(comments))}
	for _, c := range comments {
		resp.Comments = append(resp.Comments, CommentResponseInner{
			ID:        c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Body:      c.Body,
			Author: CommentAuthor{
				Username:  c.Author.Username,
				Bio:       c.Author.Bio,
				Image:     c.Author.Image,
				Following: c.Author.Following,
			},
		})
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

type TagsResponse struct {
	Tags []string `json:"tags"`
}

func (h *Handler) GetTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tags, err := h.tagService.GetTags(r.Context())
	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(TagsResponse{Tags: tags})
}
