package domain

import "context"

type commentRepo interface {
	InsertComment(ctx context.Context, authorID int, articleSlug string, c *CreateComment) (*Comment, error)
	GetCommentsByArticleSlug(ctx context.Context, articleSlug string, viewerID int) ([]*Comment, error)
	DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error
}

type CommentController struct {
	repo commentRepo
}

func NewCommentController(r commentRepo) *CommentController {
	return &CommentController{repo: r}
}

func (c *CommentController) CreateComment(ctx context.Context, authorID int, articleSlug string, comment *CreateComment) (*Comment, error) {
	if comment.Body == "" {
		return nil, NewValidationError("body", blankFieldErrMsg)
	}
	return c.repo.InsertComment(ctx, authorID, articleSlug, comment)
}

func (c *CommentController) GetComments(ctx context.Context, articleSlug string, viewerID int) ([]*Comment, error) {
	return c.repo.GetCommentsByArticleSlug(ctx, articleSlug, viewerID)
}

func (c *CommentController) DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error {
	return c.repo.DeleteComment(ctx, callerID, articleSlug, commentID)
}
