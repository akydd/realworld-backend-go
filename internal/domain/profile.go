package domain

import "context"

type profileRepo interface {
	GetProfileByUsername(ctx context.Context, profileUsername string, viewerID int) (*Profile, error)
	FollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error)
	UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error)
}

type ProfileController struct {
	repo profileRepo
}

func NewProfileController(r profileRepo) *ProfileController {
	return &ProfileController{repo: r}
}

func (c *ProfileController) GetProfile(ctx context.Context, profileUsername string, viewerID int) (*Profile, error) {
	return c.repo.GetProfileByUsername(ctx, profileUsername, viewerID)
}

func (c *ProfileController) FollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error) {
	return c.repo.FollowUser(ctx, followerID, followeeUsername)
}

func (c *ProfileController) UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error) {
	return c.repo.UnfollowUser(ctx, followerID, followeeUsername)
}
