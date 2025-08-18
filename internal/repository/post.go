package repository

import (
	"database/sql"
	"fmt"

	"lewdarchive/internal/model"
)

type PostRepository struct {
	db *sql.DB
}

func NewPostRepository(db *sql.DB) *PostRepository {
	return &PostRepository{db: db}
}

func (r *PostRepository) ExistsByHash(hash string) (bool, error) {
	var exists bool
	err := r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM posts WHERE hash = ?)", hash).Scan(&exists)
	return exists, err
}

func (r *PostRepository) Create(post *model.Post) error {
	query := `
		INSERT INTO posts (site_url, entry_id, hash, title, url, published_at, content, author, category_id, category_title)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	_, err := r.db.Exec(query,
		post.SiteURL,
		post.EntryID,
		post.Hash,
		post.Title,
		post.URL,
		post.PublishedAt,
		post.Content,
		post.Author,
		post.CategoryID,
		post.CategoryTitle,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}
	
	return nil
}

func (r *PostRepository) GetByHash(hash string) (*model.Post, error) {
	query := `
		SELECT id, site_url, entry_id, hash, title, url, published_at, content, author, category_id, category_title
		FROM posts WHERE hash = ?
	`
	
	post := &model.Post{}
	err := r.db.QueryRow(query, hash).Scan(
		&post.ID,
		&post.SiteURL,
		&post.EntryID,
		&post.Hash,
		&post.Title,
		&post.URL,
		&post.PublishedAt,
		&post.Content,
		&post.Author,
		&post.CategoryID,
		&post.CategoryTitle,
	)
	
	if err != nil {
		return nil, err
	}
	
	return post, nil
}