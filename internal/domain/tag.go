package domain

import "context"

type tagRepo interface {
	GetAllTags(ctx context.Context) ([]string, error)
}

type TagController struct {
	repo tagRepo
}

func NewTagController(r tagRepo) *TagController {
	return &TagController{repo: r}
}

func (c *TagController) GetTags(ctx context.Context) ([]string, error) {
	return c.repo.GetAllTags(ctx)
}
