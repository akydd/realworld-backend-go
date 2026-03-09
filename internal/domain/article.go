package domain

import (
	"context"
	"regexp"
	"strings"
)

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

func generateSlug(title string) string {
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

type articleRepo interface {
	InsertArticle(ctx context.Context, authorID int, slug string, a *CreateArticle) (*Article, error)
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

	slug := generateSlug(a.Title)

	return c.repo.InsertArticle(ctx, authorID, slug, a)
}
