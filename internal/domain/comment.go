package domain

import "context"

type commentRepo interface {
	InsertComment(ctx context.Context, authorID int, articleSlug string, c *CreateComment) (*Comment, error)
	GetCommentsByArticleSlug(ctx context.Context, articleSlug string, viewerID int) ([]*Comment, error)
	DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error
}

// CommentController implements the comment management use-cases of the domain.
type CommentController struct {
	repo commentRepo
}

// NewCommentController creates a CommentController backed by the given repository.
func NewCommentController(r commentRepo) *CommentController {
	return &CommentController{repo: r}
}

// CreateComment validates the comment body and persists a new comment on the specified article.
func (c *CommentController) CreateComment(ctx context.Context, authorID int, articleSlug string, comment *CreateComment) (*Comment, error) {
	if comment.Body == "" {
		return nil, NewValidationError("body", blankFieldErrMsg)
	}
	return c.repo.InsertComment(ctx, authorID, articleSlug, comment)
}

// GetComments retrieves all comments for the article identified by articleSlug.
func (c *CommentController) GetComments(ctx context.Context, articleSlug string, viewerID int) ([]*Comment, error) {
	return c.repo.GetCommentsByArticleSlug(ctx, articleSlug, viewerID)
}

// DeleteComment removes the comment identified by commentID from the article if the caller is the comment author.
func (c *CommentController) DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error {
	return c.repo.DeleteComment(ctx, callerID, articleSlug, commentID)
}
