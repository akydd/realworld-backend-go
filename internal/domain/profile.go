package domain

import "context"

type profileRepo interface {
	GetProfileByUsername(ctx context.Context, profileUsername string, viewerUsername string) (*Profile, error)
	FollowUser(ctx context.Context, followerUsername, followeeUsername string) error
	UnfollowUser(ctx context.Context, followerUsername, followeeUsername string) error
}

type ProfileController struct {
	repo profileRepo
}

func NewProfileController(r profileRepo) *ProfileController {
	return &ProfileController{repo: r}
}

func (c *ProfileController) GetProfile(ctx context.Context, profileUsername string, viewerUsername string) (*Profile, error) {
	return c.repo.GetProfileByUsername(ctx, profileUsername, viewerUsername)
}

func (c *ProfileController) FollowUser(ctx context.Context, followerUsername, followeeUsername string) (*Profile, error) {
	if err := c.repo.FollowUser(ctx, followerUsername, followeeUsername); err != nil {
		return nil, err
	}
	return c.repo.GetProfileByUsername(ctx, followeeUsername, followerUsername)
}

func (c *ProfileController) UnfollowUser(ctx context.Context, followerUsername, followeeUsername string) (*Profile, error) {
	if err := c.repo.UnfollowUser(ctx, followerUsername, followeeUsername); err != nil {
		return nil, err
	}
	return c.repo.GetProfileByUsername(ctx, followeeUsername, followerUsername)
}
