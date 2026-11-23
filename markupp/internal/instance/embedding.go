package instance

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
)

// Estados do modelo que o indexador de embedding informa.
const (
	ModelDownloading = "downloading"
	ModelReady       = "ready"
	ModelError       = "error"
)

// maxEmbeddingField limita URL, modelo e chave, que chegam de fora.
const maxEmbeddingField = 2048

// Erros da escolha do provedor de embedding.
var (
	ErrHostLocked       = errors.New("the embedding provider is set by the host configuration")
	ErrInvalidEmbedding = errors.New("invalid embedding provider choice")
	ErrAPIKeyRequired   = errors.New("embedding API key required to change the provider or URL")
)

// EmbeddingChoice é a escolha guardada no banco, com a chave do endpoint remoto cifrada.
type EmbeddingChoice struct {
	Provider     string
	URL          string
	Model        string
	SealedAPIKey string
	UpdatedAt    time.Time
}

// ModelStatus é o estado do modelo informado pelo indexador de embedding.
type ModelStatus struct {
	State     string     `json:"state"`
	Error     string     `json:"error"`
	ChangedAt *time.Time `json:"changed_at"`
}

// EmbeddingRecord é a linha da instância: a escolha, a trava de quem hospeda e o
// estado do modelo, os dois últimos escritos pelo indexador de embedding.
type EmbeddingRecord struct {
	Choice       EmbeddingChoice
	HostLocked   bool
	HostProvider string
	Model        ModelStatus
}

// EmbeddingStore persiste o registro do embedding da instância.
type EmbeddingStore interface {
	LoadEmbedding(ctx context.Context) (EmbeddingRecord, error)
	SaveEmbeddingChoice(ctx context.Context, choice EmbeddingChoice) error
	// ResealAPIKey troca a chave selada old por fresh, sem mudar a escolha nem a revisão.
	ResealAPIKey(ctx context.Context, old, fresh string) error
}

// Sealer cifra e decifra a chave do endpoint remoto. Open diz também, em stale, que a
// chave abriu com uma chave de cifra antiga e precisa de novo selo.
type Sealer interface {
	Seal(plain string) (string, error)
	Open(sealed string) (plain string, stale bool, err error)
}

// EmbeddingUpdate é a escolha pedida. APIKey nil mantém a chave guardada, e vazia a apaga.
type EmbeddingUpdate struct {
	Provider string
	URL      string
	Model    string
	APIKey   *string
}

// EmbeddingView é o que a API mostra: nunca a chave, só se há uma.
type EmbeddingView struct {
	Provider     string       `json:"provider"`
	URL          string       `json:"url"`
	Model        string       `json:"model"`
	APIKeySet    bool         `json:"api_key_set"`
	Locked       bool         `json:"locked"`
	HostProvider string       `json:"host_provider"`
	ModelStatus  *ModelStatus `json:"model_status"`
}

// PlainEmbeddingChoice é a escolha com a chave decifrada, só para o indexador de
// embedding. UpdatedAt muda a cada gravação, mesmo sem mudar valor.
type PlainEmbeddingChoice struct {
	Provider  string
	URL       string
	Model     string
	APIKey    string
	UpdatedAt time.Time
}

// EmbeddingService lê e grava a escolha do provedor de embedding da instância.
type EmbeddingService struct {
	store  EmbeddingStore
	sealer Sealer
}

// NewEmbeddingService monta o serviço sobre o banco e a cifra da chave.
func NewEmbeddingService(store EmbeddingStore, sealer Sealer) *EmbeddingService {
	return &EmbeddingService{store: store, sealer: sealer}
}

// Settings devolve a escolha, a trava e o estado do modelo, e sela de novo com a chave de
// cifra ativa a chave guardada com uma antiga.
func (s *EmbeddingService) Settings(ctx context.Context) (EmbeddingView, error) {
	record, err := s.store.LoadEmbedding(ctx)
	if err != nil {
		return EmbeddingView{}, err
	}
	if err := s.resealStaleKey(ctx, record.Choice.SealedAPIKey); err != nil {
		return EmbeddingView{}, err
	}
	return viewOf(record), nil
}

// resealStaleKey sela de novo a chave que abriu com uma chave de cifra antiga. A que não
// abre com nenhuma fica como está: só digitando a chave de novo.
func (s *EmbeddingService) resealStaleKey(ctx context.Context, sealed string) error {
	if sealed == "" {
		return nil
	}
	plain, stale, err := s.sealer.Open(sealed)
	if err != nil || !stale {
		return nil //nolint:nilerr // a chave ilegível não impede ler a escolha; o painel pede para digitá-la de novo
	}
	fresh, err := s.sealer.Seal(plain)
	if err != nil {
		return fmt.Errorf("reseal embedding API key: %w", err)
	}
	return s.store.ResealAPIKey(ctx, sealed, fresh)
}

// Update confere e grava a escolha, cifrando a chave nova. Recusa com a escolha travada
// por quem hospeda.
func (s *EmbeddingService) Update(ctx context.Context, update EmbeddingUpdate) (EmbeddingView, error) {
	update = trimUpdate(update)
	if err := checkUpdate(update); err != nil {
		return EmbeddingView{}, err
	}
	record, err := s.store.LoadEmbedding(ctx)
	if err != nil {
		return EmbeddingView{}, err
	}
	if record.HostLocked {
		return EmbeddingView{}, ErrHostLocked
	}
	if err := checkKeyReuse(update, record.Choice); err != nil {
		return EmbeddingView{}, err
	}
	choice, err := s.choiceOf(update, record.Choice.SealedAPIKey)
	if err != nil {
		return EmbeddingView{}, err
	}
	if err := s.store.SaveEmbeddingChoice(ctx, choice); err != nil {
		return EmbeddingView{}, err
	}
	return s.Settings(ctx)
}

// PlainChoice devolve a escolha guardada com a chave decifrada.
func (s *EmbeddingService) PlainChoice(ctx context.Context) (PlainEmbeddingChoice, error) {
	record, err := s.store.LoadEmbedding(ctx)
	if err != nil {
		return PlainEmbeddingChoice{}, err
	}
	c := record.Choice
	plain := PlainEmbeddingChoice{Provider: c.Provider, URL: c.URL, Model: c.Model, UpdatedAt: c.UpdatedAt}
	if c.SealedAPIKey == "" {
		return plain, nil
	}
	if plain.APIKey, _, err = s.sealer.Open(c.SealedAPIKey); err != nil {
		return PlainEmbeddingChoice{}, fmt.Errorf("open embedding API key, expected the same key-encryption secret that sealed it: %w", err)
	}
	return plain, nil
}

func (s *EmbeddingService) choiceOf(update EmbeddingUpdate, sealedKey string) (EmbeddingChoice, error) {
	choice := EmbeddingChoice{Provider: update.Provider, URL: update.URL, Model: update.Model, SealedAPIKey: sealedKey}
	if update.APIKey == nil {
		return choice, nil
	}
	choice.SealedAPIKey = ""
	if *update.APIKey == "" {
		return choice, nil
	}
	sealed, err := s.sealer.Seal(*update.APIKey)
	if err != nil {
		return EmbeddingChoice{}, fmt.Errorf("seal embedding API key: %w", err)
	}
	choice.SealedAPIKey = sealed
	return choice, nil
}

// checkKeyReuse só deixa reaproveitar a chave guardada com o mesmo provedor e a mesma URL:
// senão a chave iria no Authorization para um host que quem a digitou não escolheu.
func checkKeyReuse(u EmbeddingUpdate, stored EmbeddingChoice) error {
	if u.APIKey != nil || stored.SealedAPIKey == "" {
		return nil
	}
	if u.Provider == stored.Provider && u.URL == stored.URL {
		return nil
	}
	return fmt.Errorf("%w: provider %q and url %q differ from the stored provider %q and url %q, expected api_key sent again (empty to remove it)",
		ErrAPIKeyRequired, u.Provider, u.URL, stored.Provider, stored.URL)
}

func viewOf(record EmbeddingRecord) EmbeddingView {
	c := record.Choice
	view := EmbeddingView{Provider: c.Provider, URL: c.URL, Model: c.Model, APIKeySet: c.SealedAPIKey != "",
		Locked: record.HostLocked, HostProvider: record.HostProvider}
	if record.Model.State != "" {
		status := record.Model
		view.ModelStatus = &status
	}
	return view
}

func trimUpdate(u EmbeddingUpdate) EmbeddingUpdate {
	u.Provider, u.URL, u.Model = strings.TrimSpace(u.Provider), strings.TrimSpace(u.URL), strings.TrimSpace(u.Model)
	if u.APIKey != nil {
		key := strings.TrimSpace(*u.APIKey)
		u.APIKey = &key
	}
	return u
}

// checkUpdate confere o provedor, os campos que o remoto exige e o tamanho de cada campo.
func checkUpdate(u EmbeddingUpdate) error {
	fields := map[string]string{"url": u.URL, "model": u.Model}
	if u.APIKey != nil {
		fields["api_key"] = *u.APIKey
	}
	for name, value := range fields {
		if len(value) > maxEmbeddingField {
			return fmt.Errorf("%w: %s has %d bytes, expected at most %d", ErrInvalidEmbedding, name, len(value), maxEmbeddingField)
		}
	}
	switch u.Provider {
	case config.EmbeddingNone, config.EmbeddingLocal:
		return nil
	case config.EmbeddingOpenAICompat:
		return checkRemote(u)
	}
	return fmt.Errorf("%w: provider %q, expected one of %s, %s, %s", ErrInvalidEmbedding, u.Provider,
		config.EmbeddingNone, config.EmbeddingOpenAICompat, config.EmbeddingLocal)
}

func checkRemote(u EmbeddingUpdate) error {
	parsed, err := url.Parse(u.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%w: url %q, expected an http:// or https:// base URL such as https://api.openai.com/v1 with provider %s",
			ErrInvalidEmbedding, u.URL, config.EmbeddingOpenAICompat)
	}
	if u.Model == "" {
		return fmt.Errorf("%w: model is empty, expected the model name with provider %s", ErrInvalidEmbedding, config.EmbeddingOpenAICompat)
	}
	return nil
}
