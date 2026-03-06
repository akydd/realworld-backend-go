package domain

import "context"

type profileRepo interface {
	GetProfileByUsername(ctx context.Context, profileUsername string) (*Profile, error)
}

type ProfileController struct {
	repo profileRepo
}

func NewProfileController(r profileRepo) *ProfileController {
	return &ProfileController{repo: r}
}

func (c *ProfileController) GetProfile(ctx context.Context, profileUsername string) (*Profile, error) {
	return c.repo.GetProfileByUsername(ctx, profileUsername)
}
