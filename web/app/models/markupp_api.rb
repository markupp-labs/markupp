require "net/http"

# Conexão com a API REST do servidor markupp, com a credencial de quem chama: token de acesso e tenant ativo.
class MarkuppApi
  class Error < StandardError; end
  class NotFound < Error; end
  class Conflict < Error; end
  class DuplicatePath < Error; end
  class InvalidPath < Error; end
  class Unauthorized < Error; end
  class RefreshRejected < Error; end
  class InvalidCredentials < Error; end
  # Levantada pelo painel, não pela API: a renovação do token falhou.
  class SessionExpired < Error; end
  class InvalidRequest < Error; end
  class Forbidden < Error; end
  class LocalAuthDisabled < Forbidden; end
  # viewer tentou escrever nota.
  class ReadOnly < Forbidden; end
  # O tenant do cabeçalho não serve: não é da conta (403), a conta não tem tenant (403) ou faltou escolher (400).
  class TenantUnavailable < Forbidden; end

  NETWORK_ERRORS = [ SystemCallError, SocketError, Net::OpenTimeout, Net::ReadTimeout ].freeze
  # Erros que o painel trata, pelo campo error do corpo; 409 tem dois motivos. Id recusado é nota que não existe.
  ERRORS_BY_CODE = {
    "conflict" => Conflict, "duplicate_path" => DuplicatePath, "invalid_path" => InvalidPath, "invalid_id" => NotFound,
    "unauthorized" => Unauthorized, "invalid_refresh_token" => RefreshRejected, "invalid_credentials" => InvalidCredentials,
    "invalid_request" => InvalidRequest, "forbidden" => Forbidden, "local_auth_disabled" => LocalAuthDisabled,
    "read_only" => ReadOnly, "tenant_forbidden" => TenantUnavailable, "no_tenant" => TenantUnavailable, "tenant_required" => TenantUnavailable
  }.freeze
  # Para o status cujo corpo não traz código conhecido.
  ERRORS_BY_STATUS = { "401" => Unauthorized, "403" => Forbidden, "404" => NotFound }.freeze

  def initialize(base_url, access_token: nil, tenant_id: nil)
    @base_url = base_url
    @access_token = access_token
    @tenant_id = tenant_id
  end

  def get(path) = request(Net::HTTP::Get, path)

  def put(path, body) = request(Net::HTTP::Put, path, body)

  def patch(path, body) = request(Net::HTTP::Patch, path, body)

  def post(path, body) = request(Net::HTTP::Post, path, body)

  def delete(path) = request(Net::HTTP::Delete, path)

  private
    def request(verb, path, body = nil)
      uri = URI.join(@base_url, path)
      response = Net::HTTP.start(uri.host, uri.port, use_ssl: uri.scheme == "https") { |http| http.request(build_request(verb, uri, body)) }
      parse_response(response, "#{verb::METHOD} #{path}")
    rescue *NETWORK_ERRORS => error
      raise Error, "#{verb::METHOD} #{path} could not reach #{@base_url}: #{error.message}"
    end

    def build_request(verb, uri, body)
      http_request = verb.new(uri)
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
