package db

import (
	"context"
	"database/sql"
	"errors"

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
