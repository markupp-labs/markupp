# HTTP Basic do resource MCP da instância (ADR-0050), com que o painel se apresenta ao Go como o relay da leitura pública:
# só assim o Go conta o limite por IP pelo visitante do X-Forwarded-For, e não pelo painel. Usuário e senha vão
# codificados como form (RFC 6749, seção 2.3.1), como o servidor MCP manda. O segredo vem da variável ou do arquivo que o
# Go escreve na primeira subida; enquanto o arquivo não existe, não há credencial e o limite fica no IP do painel.
class PublicRelayCredential
  # Sem a URL do MCP a instância não serve o MCP, e não há resource para o painel usar.
  def self.from_env(env)
    resource_url = env["MARKUPP_MCP_URL"].presence
    return unless resource_url

    new(resource_url, secret: env["MARKUPP_MCP_RESOURCE_SECRET"].presence, secret_path: env["MARKUPP_MCP_SECRET_PATH"].presence)
  end

  def initialize(resource_url, secret: nil, secret_path: nil, file_reader: File)
    @resource_url = resource_url
    @secret = secret
    @secret_path = secret_path
    @file_reader = file_reader
  end

  def authorization
    secret = current_secret
    return unless secret

    credentials = "#{URI.encode_www_form_component(@resource_url)}:#{URI.encode_www_form_component(secret)}"
    "Basic #{Base64.strict_encode64(credentials)}"
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
