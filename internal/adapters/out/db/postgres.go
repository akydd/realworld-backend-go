package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"embed"
	"fmt"
	"realworld-backend-go/internal/domain"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Postgres struct {
	db *sqlx.DB
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

func validateConfig(c *DBConfig) error {
	if c.Host == "" {
		return errors.New("db config must contain a host")
	}

	if c.Port == "" {
		return errors.New("db config must contain a port")
	}

	if c.User == "" {
		return errors.New("db config must contain a user")
	}

	if c.Password == "" {
		return errors.New("db config must contain a password")
	}

	if c.Name == "" {
		return errors.New("db config must contain a name")
	}
	return nil
}

func New(config *DBConfig) (*Postgres, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	db, err := sqlx.Open("postgres",
		fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			config.Host, config.Port, config.User, config.Password, config.Name))
	if err != nil {
		return nil, err
	}

	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return nil, err
	}

	if err := goose.Up(db.DB, "migrations"); err != nil {
		return nil, err
	}

	return &Postgres{
		db: db,
	}, nil
}

type user struct {
	ID       int            `db:"id"`
	Email    string         `db:"email"`
	Username string         `db:"username"`
	Bio      sql.NullString `db:"bio"`
	Image    sql.NullString `db:"image"`
}

type userWithPassword struct {
	user
	Password string `db:"password"`
}

func convertUser(u user) domain.User {
	d := domain.User{
		ID:       u.ID,
		Username: u.Username,
		Email:    u.Email,
	}

	if u.Bio.Valid {
		s := u.Bio.String
		d.Bio = &s
	}

	if u.Image.Valid {
		s := u.Image.String
		d.Image = &s
	}

	return d
}

type profileRow struct {
	Username  string         `db:"username"`
	Bio       sql.NullString `db:"bio"`
	Image     sql.NullString `db:"image"`
	Following bool           `db:"following"`
}

func convertProfile(r profileRow) *domain.Profile {
	p := &domain.Profile{
		Username:  r.Username,
		Following: r.Following,
	}
	if r.Bio.Valid {
		s := r.Bio.String
		p.Bio = &s
	}
	if r.Image.Valid {
		s := r.Image.String
		p.Image = &s
	}
	return p
}

func (p *Postgres) GetProfileByUsername(ctx context.Context, profileUsername string, viewerID int) (*domain.Profile, error) {
	query := `
		SELECT u.username, u.bio, u.image,
			CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following
		FROM users u
		LEFT JOIN follows f ON f.followee_id = u.id AND f.follower_id = $2
		WHERE u.username = $1`
	var row profileRow

	err := p.db.QueryRowxContext(ctx, query, profileUsername, viewerID).StructScan(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ProfileNotFoundError{}
		}
		return nil, err
	}

	return convertProfile(row), nil
}

func (p *Postgres) FollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error) {
	var followeeID int
	err := p.db.QueryRowxContext(ctx, "SELECT id FROM users WHERE username = $1", followeeUsername).Scan(&followeeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ProfileNotFoundError{}
		}
		return nil, err
	}

	_, err = p.db.ExecContext(ctx,
		"INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		followerID, followeeID)
	if err != nil {
		return nil, err
	}

	return p.GetProfileByUsername(ctx, followeeUsername, followerID)
}

func (p *Postgres) UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error) {
	var followeeID int
	err := p.db.QueryRowxContext(ctx, "SELECT id FROM users WHERE username = $1", followeeUsername).Scan(&followeeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ProfileNotFoundError{}
		}
		return nil, err
	}

	_, err = p.db.ExecContext(ctx,
		"DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2",
		followerID, followeeID)
	if err != nil {
		return nil, err
	}

	return p.GetProfileByUsername(ctx, followeeUsername, followerID)
}

func (p *Postgres) GetUserByID(ctx context.Context, id int) (*domain.User, error) {
	query := "SELECT id, username, email, bio, image FROM users WHERE id = $1"
	var dbUser user

	err := p.db.QueryRowxContext(ctx, query, id).StructScan(&dbUser)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.CredentialsError{}
		}
		return nil, err
	}

	u := convertUser(dbUser)
	return &u, nil
}

func (p *Postgres) GetFullUserByID(ctx context.Context, id int) (*domain.User, string, error) {
	query := "SELECT id, username, email, bio, image, password FROM users WHERE id = $1"
	var dbUser userWithPassword

	err := p.db.QueryRowxContext(ctx, query, id).StructScan(&dbUser)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", &domain.CredentialsError{}
		}
		return nil, "", err
	}

	u := convertUser(dbUser.user)
	return &u, dbUser.Password, nil
}

func (p *Postgres) UpdateUser(ctx context.Context, userID int, u *domain.UpdateUserData) (*domain.User, error) {
	query := `UPDATE users SET email=$1, username=$2, password=$3, bio=$4, image=$5 WHERE id=$6 RETURNING id, username, email, bio, image`
	var dbUser user

	err := p.db.QueryRowxContext(ctx, query, u.Email, u.Username, u.Password, u.Bio, u.Image, userID).StructScan(&dbUser)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			switch pqErr.Constraint {
			case "users_email_unique":
				return nil, domain.NewDuplicateError("email")
			case "users_username_unique":
				return nil, domain.NewDuplicateError("username")
			}
		}
		return nil, err
	}

	updated := convertUser(dbUser)
	return &updated, nil
}

func (p *Postgres) GetUserByEmail(ctx context.Context, email string) (*domain.User, string, error) {
	query := "SELECT id, username, email, bio, image, password FROM users WHERE email = $1"
	var dbUser userWithPassword

	err := p.db.QueryRowxContext(ctx, query, email).StructScan(&dbUser)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", &domain.CredentialsError{}
		}
		return nil, "", err
	}

	user := convertUser(dbUser.user)
	return &user, dbUser.Password, nil
}

type articleRow struct {
	ID          int       `db:"id"`
	Slug        string    `db:"slug"`
	Title       string    `db:"title"`
	Description string    `db:"description"`
	Body        string    `db:"body"`
	AuthorID    int       `db:"author_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (p *Postgres) InsertArticle(ctx context.Context, authorID int, slug string, a *domain.CreateArticle) (*domain.Article, error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	query := `
		INSERT INTO articles (slug, title, description, body, author_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, slug, title, description, body, author_id, created_at, updated_at`
	var row articleRow

	err = tx.QueryRowxContext(ctx, query, slug, a.Title, a.Description, a.Body, authorID).StructScan(&row)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			switch pqErr.Constraint {
			case "articles_title_unique", "articles_slug_unique":
				return nil, domain.NewDuplicateError("title")
			}
		}
		return nil, err
	}

	if len(a.TagList) > 0 {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO tags (name) SELECT unnest($1::text[]) ON CONFLICT (name) DO NOTHING`,
			pq.Array(a.TagList))
		if err != nil {
			return nil, err
		}

		var tagIDs []int
		err = tx.SelectContext(ctx, &tagIDs,
			`SELECT id FROM tags WHERE name = ANY($1) ORDER BY array_position($1::text[], name)`,
			pq.Array(a.TagList))
		if err != nil {
			return nil, err
		}

		for _, tagID := range tagIDs {
			_, err = tx.ExecContext(ctx,
				`INSERT INTO article_tags (article_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				row.ID, tagID)
			if err != nil {
				return nil, err
			}
		}
	}

	var authorRow struct {
		Username string         `db:"username"`
		Bio      sql.NullString `db:"bio"`
		Image    sql.NullString `db:"image"`
	}
	err = tx.QueryRowxContext(ctx, "SELECT username, bio, image FROM users WHERE id = $1", authorID).StructScan(&authorRow)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	author := domain.Profile{Username: authorRow.Username, Following: false}
	if authorRow.Bio.Valid {
		s := authorRow.Bio.String
		author.Bio = &s
	}
	if authorRow.Image.Valid {
		s := authorRow.Image.String
		author.Image = &s
	}

	tagList := a.TagList
	if tagList == nil {
		tagList = []string{}
	}

	return &domain.Article{
		Slug:           row.Slug,
		Title:          row.Title,
		Description:    row.Description,
		Body:           row.Body,
		TagList:        tagList,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Favorited:      false,
		FavoritesCount: 0,
		Author:         author,
	}, nil
}

type articleWithTagsRow struct {
	Slug           string         `db:"slug"`
	Title          string         `db:"title"`
	Description    string         `db:"description"`
	Body           string         `db:"body"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
	AuthorUsername string         `db:"author_username"`
	AuthorBio      sql.NullString `db:"author_bio"`
	AuthorImage    sql.NullString `db:"author_image"`
	Following      bool           `db:"following"`
	TagList        pq.StringArray `db:"tag_list"`
	Favorited      bool           `db:"favorited"`
	FavoritesCount int            `db:"favorites_count"`
}

func (p *Postgres) GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*domain.Article, error) {
	query := `
		SELECT
			a.slug,
			a.title,
			a.description,
			a.body,
			a.created_at,
			a.updated_at,
			u.username  AS author_username,
			u.bio       AS author_bio,
			u.image     AS author_image,
			CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following,
			COALESCE(ARRAY_AGG(t.name ORDER BY t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tag_list,
			CASE WHEN fav.user_id IS NOT NULL THEN true ELSE false END AS favorited,
			(SELECT COUNT(*) FROM article_favorites WHERE article_id = a.id) AS favorites_count
		FROM articles a
		JOIN users u ON u.id = a.author_id
		LEFT JOIN follows f ON f.followee_id = a.author_id AND f.follower_id = $2
		LEFT JOIN article_tags at ON at.article_id = a.id
		LEFT JOIN tags t ON t.id = at.tag_id
		LEFT JOIN article_favorites fav ON fav.article_id = a.id AND fav.user_id = $2
		WHERE a.slug = $1
		GROUP BY a.id, u.username, u.bio, u.image, f.follower_id, fav.user_id`

	var row articleWithTagsRow
	err := p.db.QueryRowxContext(ctx, query, slug, viewerID).StructScan(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ArticleNotFoundError{}
		}
		return nil, err
	}

	author := domain.Profile{
		Username:  row.AuthorUsername,
		Following: row.Following,
	}
	if row.AuthorBio.Valid {
		s := row.AuthorBio.String
		author.Bio = &s
	}
	if row.AuthorImage.Valid {
		s := row.AuthorImage.String
		author.Image = &s
	}

	tagList := []string(row.TagList)
	if tagList == nil {
		tagList = []string{}
	}

	return &domain.Article{
		Slug:           row.Slug,
		Title:          row.Title,
		Description:    row.Description,
		Body:           row.Body,
		TagList:        tagList,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Favorited:      row.Favorited,
		FavoritesCount: row.FavoritesCount,
		Author:         author,
	}, nil
}

func (p *Postgres) FavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error) {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO article_favorites (user_id, article_id)
		 SELECT $1, id FROM articles WHERE slug = $2
		 ON CONFLICT DO NOTHING`,
		userID, slug)
	if err != nil {
		return nil, err
	}
	return p.GetArticleBySlug(ctx, slug, userID)
}

func (p *Postgres) UnfavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error) {
	_, err := p.db.ExecContext(ctx,
		`DELETE FROM article_favorites
		 WHERE user_id = $1 AND article_id = (SELECT id FROM articles WHERE slug = $2)`,
		userID, slug)
	if err != nil {
		return nil, err
	}
	return p.GetArticleBySlug(ctx, slug, userID)
}

func (p *Postgres) UpdateArticle(ctx context.Context, callerID int, slug string, u *domain.UpdateArticle) (*domain.Article, error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var cur struct {
		ID          int    `db:"id"`
		AuthorID    int    `db:"author_id"`
		Title       string `db:"title"`
		Description string `db:"description"`
		Body        string `db:"body"`
	}
	err = tx.QueryRowxContext(ctx,
		`SELECT id, author_id, title, description, body FROM articles WHERE slug = $1`,
		slug).StructScan(&cur)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &domain.ArticleNotFoundError{}
		}
		return nil, err
	}

	if cur.AuthorID != callerID {
		return nil, &domain.CredentialsError{}
	}

	newTitle := cur.Title
	newDescription := cur.Description
	newBody := cur.Body
	newSlug := slug

	if u.Title != nil {
		newTitle = *u.Title
		newSlug = domain.GenerateSlug(newTitle)
	}
	if u.Description != nil {
		newDescription = *u.Description
	}
	if u.Body != nil {
		newBody = *u.Body
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE articles SET slug=$1, title=$2, description=$3, body=$4, updated_at=now() WHERE id=$5`,
		newSlug, newTitle, newDescription, newBody, cur.ID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			switch pqErr.Constraint {
			case "articles_title_unique", "articles_slug_unique":
				return nil, domain.NewDuplicateError("title")
			}
		}
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return p.GetArticleBySlug(ctx, newSlug, callerID)
}

func (p *Postgres) GetAllTags(ctx context.Context) ([]string, error) {
	var tags []string
	err := p.db.SelectContext(ctx, &tags, `SELECT name FROM tags ORDER BY name`)
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

func (p *Postgres) InsertUser(ctx context.Context, u *domain.RegisterUser) (*domain.User, error) {
	query := "insert into users (username, email, password) values ($1, $2, $3) returning id, username, email, bio, image"
	var dbUser user

	err := p.db.QueryRowxContext(ctx, query, u.Username, u.Email, u.Password).StructScan(&dbUser)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			switch pqErr.Constraint {
			case "users_email_unique":
				return nil, domain.NewDuplicateError("email")
			case "users_username_unique":
				return nil, domain.NewDuplicateError("username")
			}
		}
		return nil, err
	}

	user := convertUser(dbUser)
	return &user, nil
}
