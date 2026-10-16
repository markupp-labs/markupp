// Package notes concentra as regras de negócio das notas: validação,
// carimbo de tempo e detecção de conflito de versão.
package notes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Note é uma nota persistida, identificada por ID e endereçada por Path dentro
// do cofre VaultID. ContentSHA256 é o SHA-256 dos bytes UTF-8 de Content, em hex
// minúsculo.
type Note struct {
	ID            string
	VaultID       string
	Path          string
	Content       string
	ContentSHA256 string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// SearchResult é a projeção de uma nota devolvida pela busca: sem o conteúdo, com o
// trecho cru dele em que os termos da consulta aparecem entre U+0002 e U+0003.
type SearchResult struct {
	ID        string    `json:"id"`
	VaultID   string    `json:"vault_id"`
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updated_at"`
	Snippet   string    `json:"snippet"`
}

// Repository é a porta de persistência que o Service consome.
type Repository interface {
	DefaultVaultID(ctx context.Context) (string, error)
	Save(ctx context.Context, note Note) (Note, error)
	Update(ctx context.Context, id, path, content string, updatedAt, lastModifiedAt time.Time, force bool) (Note, error)
	Delete(ctx context.Context, id string) error
	GetNoteByID(ctx context.Context, id string) (Note, error)
	ListNotes(ctx context.Context) ([]Note, error)
	SearchNotes(ctx context.Context, query string, offset, limit int32) ([]SearchResult, error)
}

// Erros de domínio devolvidos pelo Service e traduzidos em status HTTP pela
// camada de API.
var (
	ErrInvalidPath    = errors.New("path inválido")
	ErrInvalidContent = errors.New("content inválido")
	ErrDuplicatePath  = errors.New("path já existe")
	ErrNotFound       = errors.New("nota não encontrada")
	ErrInvalidID      = errors.New("ID inválido")
	ErrConflict       = errors.New("nota foi atualizada por outro cliente")
	ErrVaultForbidden = errors.New("cofre inexistente ou sem acesso")
)

// Service aplica as regras de negócio das notas sobre um Repository.
type Service struct {
	repo           Repository
	clock          func() time.Time
	newID          func() string
	maxContentSize int64
}

// NewService monta um Service com relógio e gerador de ID reais.
func NewService(repo Repository, maxContentSize int64) *Service {
	return &Service{
		repo:           repo,
		clock:          time.Now,
		newID:          uuid.NewString,
		maxContentSize: maxContentSize,
	}
}

// GetNoteByID devolve a nota de id, ou ErrInvalidID se o id for vazio.
func (s *Service) GetNoteByID(ctx context.Context, id string) (Note, error) {
	if err := validateID(id); err != nil {
		return Note{}, err
	}
	return s.repo.GetNoteByID(ctx, id)
}

// ListNotes devolve todas as notas.
func (s *Service) ListNotes(ctx context.Context) ([]Note, error) {
	return s.repo.ListNotes(ctx)
}

// Create valida path e content e persiste uma nota nova com ID gerado no cofre
// vaultID, ou no cofre padrão do ator quando vaultID vem vazio, e devolve a nota
// como o banco a gravou, com o content_sha256 calculado lá.
func (s *Service) Create(ctx context.Context, vaultID, path, content string) (Note, error) {
	if err := validatePath(path); err != nil {
		return Note{}, err
	}
	if err := s.validateContent(content); err != nil {
		return Note{}, err
	}
	vaultID, err := s.resolveVault(ctx, vaultID)
	if err != nil {
		return Note{}, err
	}
	now := canonicalTime(s.clock())
	note := Note{ID: s.newID(), VaultID: vaultID, Path: path, Content: content, CreatedAt: now, UpdatedAt: now}
	saved, err := s.repo.Save(ctx, note)
	if err != nil {
		return Note{}, fmt.Errorf("salvar nota %q no cofre %q: %w", note.ID, vaultID, err)
	}
	return saved, nil
}

func (s *Service) resolveVault(ctx context.Context, vaultID string) (string, error) {
	if trimmed := strings.TrimSpace(vaultID); trimmed != "" {
		return trimmed, nil
	}
	return s.repo.DefaultVaultID(ctx)
}

// Update grava path e content na nota de id. Com force falso, recusa com
// ErrConflict quando lastModifiedAt não corresponde à versão armazenada. O
// updated_at gravado é sempre estritamente maior que o anterior.
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

// Delete remove a nota de id, ou devolve ErrNotFound se ela não existir.
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

// SearchNotes busca no índice léxico, normalizando offset negativo para zero e
// limit não positivo para dez.
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
