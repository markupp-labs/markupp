package notes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Note struct {
	ID        string
	Path      string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SearchResult struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Repository interface {
	Save(ctx context.Context, note Note) error
	Update(ctx context.Context, id, path, content string, updatedAt, lastModifiedAt time.Time, force bool) (Note, error)
	Delete(ctx context.Context, id string) error
	GetNoteByID(ctx context.Context, id string) (Note, error)
	ListNotes(ctx context.Context) ([]Note, error)
	SearchNotes(ctx context.Context, query string, offset, limit int32) ([]SearchResult, error)
}

var (
	ErrInvalidPath    = errors.New("path inválido")
	ErrInvalidContent = errors.New("content inválido")
	ErrDuplicatePath  = errors.New("path já existe")
	ErrNotFound       = errors.New("nota não encontrada")
	ErrInvalidID      = errors.New("ID inválido")
	ErrConflict       = errors.New("nota foi atualizada por outro cliente")
)

type Service struct {
	repo           Repository
	clock          func() time.Time
	newID          func() string
	maxContentSize int64
}

func NewService(repo Repository, maxContentSize int64) *Service {
	return &Service{
		repo:           repo,
		clock:          time.Now,
		newID:          uuid.NewString,
		maxContentSize: maxContentSize,
	}
}

func (s *Service) GetNoteByID(ctx context.Context, id string) (Note, error) {
	if err := validateID(id); err != nil {
		return Note{}, err
	}
	return s.repo.GetNoteByID(ctx, id)
}

func (s *Service) ListNotes(ctx context.Context) ([]Note, error) {
	return s.repo.ListNotes(ctx)
}

func (s *Service) Create(ctx context.Context, path, content string) (Note, error) {
	if err := validatePath(path); err != nil {
		return Note{}, err
	}
	if err := s.validateContent(content); err != nil {
		return Note{}, err
	}
	now := canonicalTime(s.clock())
	note := Note{
		ID:        s.newID(),
		Path:      path,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Save(ctx, note); err != nil {
		return Note{}, fmt.Errorf("salvar nota %q: %w", note.ID, err)
	}
	return note, nil
}

func (s *Service) Update(ctx context.Context, id, path, content string, lastModifiedAt time.Time, force bool) (Note, error) {
	if err := validateID(id); err != nil {
		return Note{}, err
	}
	if err := validatePath(path); err != nil {
		return Note{}, err
	}
	if err := s.validateContent(content); err != nil {
		return Note{}, err
	}

	atual, err := s.repo.GetNoteByID(ctx, id)
	if err != nil {
		return Note{}, err
	}

	now := strictlyAfter(atual.UpdatedAt, canonicalTime(s.clock()))
	updated, err := s.repo.Update(ctx, id, path, content, now, canonicalTime(lastModifiedAt), force)
	if err != nil {
		if errors.Is(err, ErrDuplicatePath) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) {
			return Note{}, err
		}
		return Note{}, fmt.Errorf("atualizar nota %q: %w", id, err)
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return err
		}
		return fmt.Errorf("excluir nota %q: %w", id, err)
	}
	return nil
}

func validateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidID
	}
	return nil
}

func strictlyAfter(previous, now time.Time) time.Time {
	if now.After(previous) {
		return now
	}
	return previous.Add(time.Millisecond)
}

func canonicalTime(t time.Time) time.Time {
	return t.UTC().Truncate(time.Millisecond)
}

func validatePath(path string) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return ErrInvalidPath
	}
	if len(p) > 1024 {
		return ErrInvalidPath
	}
	if strings.Contains(p, "..") {
		return ErrInvalidPath
	}
	return nil
}

func (s *Service) validateContent(content string) error {
	if int64(len(content)) > s.maxContentSize {
		return ErrInvalidContent
	}
	return nil
}

func (s *Service) SearchNotes(ctx context.Context, query string, offset, limit int) ([]SearchResult, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 10
	}

	results, err := s.repo.SearchNotes(ctx, query, int32(offset), int32(limit))
	if err != nil {
		return nil, fmt.Errorf("buscar notas: %w", err)
	}
	return results, nil
}
