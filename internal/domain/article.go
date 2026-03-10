package domain

import (
	"context"
	"regexp"
	"strings"
)

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

func GenerateSlug(title string) string {
	lower := strings.ToLower(title)
	slug := nonAlphanumRe.ReplaceAllString(lower, "-")
	return strings.Trim(slug, "-")
}

func validateCreateArticle(a *CreateArticle) error {
	if a.Title == "" {
		return NewValidationError("title", blankFieldErrMsg)
	}
	if a.Description == "" {
		return NewValidationError("description", blankFieldErrMsg)
	}
	if a.Body == "" {
		return NewValidationError("body", blankFieldErrMsg)
	}
	return nil
}

func deduplicateTags(tags []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(tags))
	for _, t := range tags {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}

func validateUpdateArticle(u *UpdateArticle) error {
	if u.Title == nil && u.Description == nil && u.Body == nil {
		return NewValidationError("article", blankFieldErrMsg)
	}
	if u.Title != nil && *u.Title == "" {
		return NewValidationError("title", blankFieldErrMsg)
	}
	return nil
}

type articleRepo interface {
	InsertArticle(ctx context.Context, authorID int, slug string, a *CreateArticle) (*Article, error)
	GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*Article, error)
	UpdateArticle(ctx context.Context, callerID int, slug string, u *UpdateArticle) (*Article, error)
	FavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error)
	UnfavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error)
	DeleteArticle(ctx context.Context, callerID int, slug string) error
}

type ArticleController struct {
	repo articleRepo
}

func NewArticleController(r articleRepo) *ArticleController {
	return &ArticleController{repo: r}
}

func (c *ArticleController) CreateArticle(ctx context.Context, authorID int, a *CreateArticle) (*Article, error) {
	if err := validateCreateArticle(a); err != nil {
		return nil, err
	}

	a.TagList = deduplicateTags(a.TagList)

	slug := GenerateSlug(a.Title)

	return c.repo.InsertArticle(ctx, authorID, slug, a)
}

func (c *ArticleController) GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*Article, error) {
	return c.repo.GetArticleBySlug(ctx, slug, viewerID)
}

func (c *ArticleController) UpdateArticle(ctx context.Context, callerID int, slug string, u *UpdateArticle) (*Article, error) {
	if err := validateUpdateArticle(u); err != nil {
		return nil, err
	}
	return c.repo.UpdateArticle(ctx, callerID, slug, u)
}

func (c *ArticleController) FavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error) {
	return c.repo.FavoriteArticle(ctx, userID, slug)
}

func (c *ArticleController) UnfavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error) {
	return c.repo.UnfavoriteArticle(ctx, userID, slug)
}

func (c *ArticleController) DeleteArticle(ctx context.Context, callerID int, slug string) error {
	return c.repo.DeleteArticle(ctx, callerID, slug)
}
