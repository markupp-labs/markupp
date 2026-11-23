# HTTP Basic com que o painel se apresenta ao Go como o relay da leitura pública (ADR-0050): só assim o Go conta o limite
# por IP pelo visitante do X-Forwarded-For, e não pelo painel. O segredo é só do painel, gerado pelo Go, e vale com o MCP
# e o OAuth desligados. Usuário e senha vão codificados como form (RFC 6749, seção 2.3.1), como o Go confere. O segredo
# vem da variável ou do arquivo que o Go escreve na primeira subida; enquanto o arquivo não existe, não há credencial e o
# limite fica no IP do painel.
class PublicRelayCredential
  # O usuário que o Go reconhece como o painel.
  PANEL_USER = "panel"

  # Sem a variável nem o arquivo, quem hospeda não deu ao painel o segredo de relay.
  def self.from_env(env)
    secret = env["MARKUPP_PANEL_RELAY_SECRET"].presence
    secret_path = env["MARKUPP_PANEL_RELAY_SECRET_PATH"].presence
    return unless secret || secret_path

    new(secret:, secret_path:)
  end

  def initialize(secret: nil, secret_path: nil, file_reader: File)
    @secret = secret
    @secret_path = secret_path
    @file_reader = file_reader
  end

  def authorization
    secret = current_secret
    return unless secret

    "Basic #{Base64.strict_encode64("#{PANEL_USER}:#{URI.encode_www_form_component(secret)}")}"
  end

  private
    def current_secret = @secret ||= written_secret

    # O erro de leitura nunca cita o conteúdo; arquivo ausente ou vazio é o Go que ainda não subiu.
    def written_secret
      return unless @secret_path

      @file_reader.read(@secret_path).strip.presence
    rescue SystemCallError
      nil
    end
end
