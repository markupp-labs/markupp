require "net/http"

# Conexão com a API REST do servidor markupp, com a credencial de quem chama: token de acesso e tenant ativo.
class MarkuppApi
  class Error < StandardError
    # Nome da classe em snake_case: a chave i18n da mensagem para quem usa o painel.
    def reason = self.class.name.demodulize.underscore
  end
  class NotFound < Error; end
  # Convite inexistente, vencido, já respondido ou de outra conta: a API não distingue.
  class InviteNotFound < NotFound; end
  # A pessoa já não é membro do tenant: removida por outro admin ou saiu.
  class MemberNotFound < NotFound; end
  # A troca deixaria o tenant sem owner.
  class LastOwner < Error; end
  class AlreadyMember < Error; end
  # Token de email recusado: convite, troca de senha ou verificação, inválido, vencido ou já usado.
  class InvalidToken < Error; end
  class AccountExists < Error; end
  class Conflict < Error; end
  class DuplicatePath < Error; end
  class InvalidPath < Error; end
  class Unauthorized < Error; end
  class RefreshRejected < Error; end
  class InvalidCredentials < Error; end
  # Levantada pelo painel, não pela API: a renovação do token falhou.
  class SessionExpired < Error; end
  class InvalidRequest < Error; end
  # O servidor não tem SMTP: os fluxos de email da conta ficam desligados.
  class EmailUnavailable < Error; end
  class EmailAlreadyVerified < Error; end
  class Forbidden < Error; end
  class LocalAuthDisabled < Forbidden; end
  # viewer tentou escrever nota.
  class ReadOnly < Forbidden; end
  # O tenant do cabeçalho não serve: não é da conta (403), a conta não tem tenant (403) ou faltou escolher (400).
  class TenantUnavailable < Forbidden; end
  # Conceder papel acima do próprio, ou mexer em quem está acima.
  class RoleAboveOwn < Forbidden; end
  # Pedido de autorização OAuth inexistente, vencido ou já decidido: a API não distingue.
  class OAuthRequestNotFound < NotFound; end
  # Cofre inexistente ou de que a conta não é membro.
  class VaultUnavailable < Forbidden; end

  # Corpo e ETag de uma leitura que pode voltar 304.
  Revision = Data.define(:body, :etag)

  # Sem eles valem os 60s do Net::HTTP, e a trava de renovação do RefreshingMarkuppApi fica presa esse tempo todo.
  # Sem repetição (max_retries 0): o Net::HTTP repetiria GET, PUT e DELETE e dobraria a espera.
  OPEN_TIMEOUT = 2
  READ_TIMEOUT = 10
  NETWORK_ERRORS = [ SystemCallError, SocketError, Net::OpenTimeout, Net::ReadTimeout ].freeze
  # Erros que o painel trata, pelo campo error do corpo; 409 tem dois motivos. Id recusado é nota que não existe.
  ERRORS_BY_CODE = {
    "conflict" => Conflict, "duplicate_path" => DuplicatePath, "invalid_path" => InvalidPath, "invalid_id" => NotFound,
    "unauthorized" => Unauthorized, "invalid_refresh_token" => RefreshRejected, "invalid_credentials" => InvalidCredentials,
    "invalid_request" => InvalidRequest, "forbidden" => Forbidden, "local_auth_disabled" => LocalAuthDisabled,
    "read_only" => ReadOnly, "tenant_forbidden" => TenantUnavailable, "no_tenant" => TenantUnavailable, "tenant_required" => TenantUnavailable,
    "invite_not_found" => InviteNotFound, "already_member" => AlreadyMember,
    "member_not_found" => MemberNotFound, "last_owner" => LastOwner, "role_above_own" => RoleAboveOwn,
    "invalid_invite" => InvalidToken, "invalid_token" => InvalidToken, "already_exists" => AccountExists,
    "email_unavailable" => EmailUnavailable, "email_already_verified" => EmailAlreadyVerified,
    "vault_forbidden" => VaultUnavailable, "request_not_found" => OAuthRequestNotFound
  }.freeze
  # Para o status cujo corpo não traz código conhecido.
  ERRORS_BY_STATUS = { "401" => Unauthorized, "403" => Forbidden, "404" => NotFound }.freeze

  def initialize(base_url, access_token: nil, tenant_id: nil, read_timeout: READ_TIMEOUT)
    @base_url = base_url
    @access_token = access_token
    @tenant_id = tenant_id
    @read_timeout = read_timeout
  end

  # Caminho da API a partir dos segmentos, cada um codificado: id com barra ou espaço não vira outra rota.
  # Os parâmetros com valor (false inclusive) vão na query.
  def self.path(*segments, **params)
    query = params.compact.to_query
    segments.map { "/#{ERB::Util.url_encode(it)}" }.join + (query.empty? ? "" : "?#{query}")
  end

  def get(path) = request(Net::HTTP::Get, path)

  # GET que revalida a cópia guardada pela ETag dela: nil no 304, quando a cópia ainda vale.
  def conditional_get(path, etag:)
    response = exchange(Net::HTTP::Get, path, nil, { "If-None-Match" => etag }.compact)
    return if response.is_a?(Net::HTTPNotModified)

    Revision.new(body: parse_response(response, "GET #{path}"), etag: response["ETag"])
  end

  def put(path, body) = request(Net::HTTP::Put, path, body)

  def patch(path, body) = request(Net::HTTP::Patch, path, body)

  def post(path, body) = request(Net::HTTP::Post, path, body)

  def delete(path) = request(Net::HTTP::Delete, path)

  private
    def request(verb, path, body = nil) = parse_response(exchange(verb, path, body), "#{verb::METHOD} #{path}")

    def exchange(verb, path, body, headers = {})
      uri = URI.join(@base_url, path)
      Net::HTTP.start(uri.host, uri.port, use_ssl: uri.scheme == "https",
                      open_timeout: OPEN_TIMEOUT, read_timeout: @read_timeout, max_retries: 0) do |http|
        http.request(build_request(verb, uri, body, headers))
      end
    rescue *NETWORK_ERRORS => error
      raise Error, "#{verb::METHOD} #{path} could not reach #{@base_url}: #{error.message}"
    end

    def build_request(verb, uri, body, headers)
      http_request = verb.new(uri, headers)
      http_request["Authorization"] = "Bearer #{@access_token}" if @access_token
      http_request["X-Markupp-Tenant"] = @tenant_id if @tenant_id
      return http_request if body.nil?

      http_request.content_type = "application/json"
      http_request.body = body.to_json
      http_request
    end

    # O 204 do DELETE chega sem corpo.
    def parse_response(response, call)
      raise error_class_for(response), "#{call} answered #{response.code}, expected 2xx: #{response.body}" unless response.is_a?(Net::HTTPSuccess)

      JSON.parse(response.body) if response.body.present?
    end

    def error_class_for(response)
      ERRORS_BY_CODE.fetch(error_code(response.body)) { ERRORS_BY_STATUS.fetch(response.code, Error) }
    end

    def error_code(body)
      JSON.parse(body)["error"]
    rescue JSON::ParserError, TypeError
      nil
    end
end
