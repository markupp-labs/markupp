package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
)

const (
	urlDeTeste           = "postgres://markupp@db:5432/markupp"
	segredoDeTesteValido = "12345678901234567890123456789012"
)

// fakeEnv faz o papel das variáveis de ambiente do processo, com a mesma
// assinatura de os.LookupEnv.
type fakeEnv map[string]string

func (f fakeEnv) Lookup(key string) (string, bool) {
	value, ok := f[key]
	return value, ok
}

func TestDefault_RetornaValoresPadrao(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, int64(50*1024*1024), cfg.MaxNoteSize)
	assert.Empty(t, cfg.DatabaseURL, "sem default: o banco precisa ser informado")
	assert.Empty(t, cfg.AllowedOrigins, "sem origem configurada o CORS fica desligado")
	assert.Equal(t, "default", cfg.DefaultTenantID)
	assert.Equal(t, 15, cfg.JWTExpirationMinutes)
	assert.Equal(t, 7, cfg.RefreshExpirationDays)
	assert.False(t, cfg.AuthRegistrationEnabled)
	assert.True(t, cfg.LocalAuthEnabled)
	assert.Empty(t, cfg.AuthSecret, "sem default: o segredo de autenticacao precisa ser informado")
}

func TestLoad_SemDatabaseURL_RetornaErroNomeandoAVariavel(t *testing.T) {
	_, err := config.Load(fakeEnv{"MARKUPP_AUTH_SECRET": segredoDeTesteValido}.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_DATABASE_URL")
}

func TestLoad_SoVariaveisObrigatorias_UsaDefaultsNoResto(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido}
	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	esperado := config.Default()
	esperado.DatabaseURL = urlDeTeste
	esperado.AuthSecret = segredoDeTesteValido
	assert.Equal(t, esperado, cfg)
}

func TestLoad_TodasAsVariaveis_LeTodosOsCampos(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL":    urlDeTeste,
		"MARKUPP_AUTH_SECRET":     segredoDeTesteValido,
		"MARKUPP_PORT":            "9090",
		"MARKUPP_MAX_NOTE_SIZE":   "1024",
		"MARKUPP_ALLOWED_ORIGINS": "https://painel.markupp.dev,http://localhost:5173",
	}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, urlDeTeste, cfg.DatabaseURL)
	assert.Equal(t, segredoDeTesteValido, cfg.AuthSecret)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, int64(1024), cfg.MaxNoteSize)
	assert.Equal(t, []string{"https://painel.markupp.dev", "http://localhost:5173"}, cfg.AllowedOrigins)
}

func TestLoad_OrigensComEspacos_AparaCadaOrigem(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL":    urlDeTeste,
		"MARKUPP_AUTH_SECRET":     segredoDeTesteValido,
		"MARKUPP_ALLOWED_ORIGINS": " https://a.dev , https://b.dev ",
	}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, []string{"https://a.dev", "https://b.dev"}, cfg.AllowedOrigins)
}

func TestLoad_OrigensVazia_MantemCORSDesligado(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido, "MARKUPP_ALLOWED_ORIGINS": ""}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Empty(t, cfg.AllowedOrigins)
}

func TestLoad_PortaNaoNumerica_RetornaErroComOValor(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido, "MARKUPP_PORT": "oitenta"}

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_PORT")
	assert.Contains(t, err.Error(), "oitenta")
}

func TestLoad_TamanhoNaoNumerico_RetornaErroComOValor(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido, "MARKUPP_MAX_NOTE_SIZE": "50MB"}

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_MAX_NOTE_SIZE")
	assert.Contains(t, err.Error(), "50MB")
}

func TestLoad_PortaForaDaFaixa_RetornaErroComOValor(t *testing.T) {
	for _, porta := range []string{"0", "-1", "65536"} {
		env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido, "MARKUPP_PORT": porta}

		_, err := config.Load(env.Lookup)

		require.Error(t, err, "porta %s devia ser recusada", porta)
		assert.Contains(t, err.Error(), porta)
		assert.Contains(t, err.Error(), "1 a 65535")
	}
}

func TestLoad_TamanhoNaoPositivo_RetornaErroComOValor(t *testing.T) {
	for _, tamanho := range []string{"0", "-5"} {
		env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido, "MARKUPP_MAX_NOTE_SIZE": tamanho}

		_, err := config.Load(env.Lookup)

		require.Error(t, err, "tamanho %s devia ser recusado", tamanho)
		assert.Contains(t, err.Error(), tamanho)
		assert.Contains(t, err.Error(), "maior que zero")
	}
}

func TestLoad_VariaveisDeAutenticacao_LeTodosOsCampos(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL":              urlDeTeste,
		"MARKUPP_AUTH_SECRET":               "segredo-customizado-de-teste-muito-seguro", // #nosec G101 -- valor ficticio de teste
		"MARKUPP_DEFAULT_TENANT_ID":         "tenant-empresa",
		"MARKUPP_JWT_EXPIRATION_MINUTES":    "30",
		"MARKUPP_REFRESH_EXPIRATION_DAYS":   "14",
		"MARKUPP_AUTH_REGISTRATION_ENABLED": "true",
		"MARKUPP_AUTH_LOCAL_ENABLED":        "false",
	}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, "segredo-customizado-de-teste-muito-seguro", cfg.AuthSecret)
	assert.Equal(t, "tenant-empresa", cfg.DefaultTenantID)
	assert.Equal(t, 30, cfg.JWTExpirationMinutes)
	assert.Equal(t, 14, cfg.RefreshExpirationDays)
	assert.True(t, cfg.AuthRegistrationEnabled)
	assert.False(t, cfg.LocalAuthEnabled)
}

func TestLoad_JWTExpirationInvalido_RetornaErroComOValor(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido, "MARKUPP_JWT_EXPIRATION_MINUTES": "zero"}

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_JWT_EXPIRATION_MINUTES")
	assert.Contains(t, err.Error(), "zero")
}

func TestLoad_RefreshExpirationInvalido_RetornaErroComOValor(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido, "MARKUPP_REFRESH_EXPIRATION_DAYS": "dez"}

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_REFRESH_EXPIRATION_DAYS")
	assert.Contains(t, err.Error(), "dez")
}

func TestLoad_SemAuthSecret_RetornaErroExigindoVariavel(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste}

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_AUTH_SECRET")
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestLoad_AuthSecretCurto_RetornaErroComTamanhoEFormato(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL": urlDeTeste,
		"MARKUPP_AUTH_SECRET":  "segredo-curto-com-menos-de-32b", // #nosec G101 -- segredo curto para teste de validacao
	}

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_AUTH_SECRET")
	assert.Contains(t, err.Error(), "30 bytes")
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestLoad_BooleanoInvalido_RetornaErroComChaveValorEFormato(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL":       urlDeTeste,
		"MARKUPP_AUTH_SECRET":        segredoDeTesteValido,
		"MARKUPP_AUTH_LOCAL_ENABLED": "sim",
	}

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_AUTH_LOCAL_ENABLED")
	assert.Contains(t, err.Error(), "sim")
	assert.Contains(t, err.Error(), "esperado booleano")
}

func TestLoad_BooleanoEmMaiusculo_InterpretaCorretamente(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL":       urlDeTeste,
		"MARKUPP_AUTH_SECRET":        segredoDeTesteValido,
		"MARKUPP_AUTH_LOCAL_ENABLED": "TRUE",
	}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.True(t, cfg.LocalAuthEnabled)
}

func TestDefault_IndexadorUsaSimpleETrintaSegundos(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, "simple", cfg.SearchConfig, "simple nao radicaliza, a saida conservadora do ADR-0017")
	assert.Equal(t, 30*time.Second, cfg.IndexInterval)
}

func TestLoad_VariaveisDoIndexador_LeConfiguracaoEIntervalo(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL":   urlDeTeste,
		"MARKUPP_AUTH_SECRET":    segredoDeTesteValido,
		"MARKUPP_SEARCH_CONFIG":  "portuguese",
		"MARKUPP_INDEX_INTERVAL": "5s",
	}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, "portuguese", cfg.SearchConfig)
	assert.Equal(t, 5*time.Second, cfg.IndexInterval)
}

func TestLoad_IntervaloInvalido_RetornaErroComOValor(t *testing.T) {
	for _, valor := range []string{"trinta", "0s", "-5s"} {
		env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido, "MARKUPP_INDEX_INTERVAL": valor}

		_, err := config.Load(env.Lookup)

		require.Error(t, err, valor)
		assert.Contains(t, err.Error(), "MARKUPP_INDEX_INTERVAL")
		assert.Contains(t, err.Error(), valor)
	}
}

func TestDefault_SemSMTP_EmailDesligado(t *testing.T) {
	cfg := config.Default()

	assert.Empty(t, cfg.SMTPURL, "sem SMTP o markupp nao envia email (ADR-0021)")
	assert.Empty(t, cfg.SMTPFrom)
}

func TestLoad_SMTPComRemetente_LeOsDois(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL": urlDeTeste,
		"MARKUPP_AUTH_SECRET":  segredoDeTesteValido,
		"MARKUPP_SMTP_URL":     "smtps://markupp@smtp.markupp.dev:465",
		"MARKUPP_SMTP_FROM":    "convites@markupp.dev",
	}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, "smtps://markupp@smtp.markupp.dev:465", cfg.SMTPURL)
	assert.Equal(t, "convites@markupp.dev", cfg.SMTPFrom)
}

func TestLoad_SMTPSemRemetente_RetornaErroNomeandoAVariavel(t *testing.T) {
	env := fakeEnv{"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido, "MARKUPP_SMTP_URL": "smtp://mailpit:1025"}

	_, err := config.Load(env.Lookup)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARKUPP_SMTP_FROM")
}

func TestLoad_SMTPForaDoFormato_RetornaErroSemVazarASenha(t *testing.T) {
	for _, valor := range []string{"http://usuario:segredo@smtp.markupp.dev", "smtp://", "mailpit:1025"} {
		env := fakeEnv{
			"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido,
			"MARKUPP_SMTP_URL": valor, "MARKUPP_SMTP_FROM": "convites@markupp.dev",
		}

		_, err := config.Load(env.Lookup)

		require.Error(t, err, valor)
		assert.Contains(t, err.Error(), "MARKUPP_SMTP_URL")
		assert.Contains(t, err.Error(), "smtp://")
		assert.NotContains(t, err.Error(), "segredo", "a mensagem de erro nao repete a senha da URL")
	}
}

func TestLoad_RemetenteComNome_Aceita(t *testing.T) {
	env := fakeEnv{
		"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido,
		"MARKUPP_SMTP_URL": "smtp://mailpit:1025", "MARKUPP_SMTP_FROM": "Markupp <convites@markupp.dev>",
	}

	cfg, err := config.Load(env.Lookup)

	require.NoError(t, err)
	assert.Equal(t, "Markupp <convites@markupp.dev>", cfg.SMTPFrom)
}

func TestLoad_RemetenteIlegivel_RetornaErroComOValor(t *testing.T) {
	for _, valor := range []string{"Markupp <convites@>", "a@b, c@d", "convites@markupp.dev>"} {
		env := fakeEnv{
			"MARKUPP_DATABASE_URL": urlDeTeste, "MARKUPP_AUTH_SECRET": segredoDeTesteValido,
			"MARKUPP_SMTP_URL": "smtp://mailpit:1025", "MARKUPP_SMTP_FROM": valor,
		}

		_, err := config.Load(env.Lookup)

		require.Error(t, err, valor)
		assert.Contains(t, err.Error(), "MARKUPP_SMTP_FROM", valor)
	}
}
