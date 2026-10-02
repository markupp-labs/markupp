require "net/http"

# Conexão com a API REST do servidor markupp. Só o Note fala com ela.
class MarkuppApi
  class Error < StandardError; end
  class NotFound < Error; end
  class Conflict < Error; end
  class DuplicatePath < Error; end
  class InvalidPath < Error; end

  NETWORK_ERRORS = [ SystemCallError, SocketError, Net::OpenTimeout, Net::ReadTimeout ].freeze
  # Erros que o painel trata, pelo campo error do corpo; 409 tem dois motivos.
  ERRORS_BY_CODE = { "conflict" => Conflict, "duplicate_path" => DuplicatePath, "invalid_path" => InvalidPath }.freeze

  def initialize(base_url)
    @base_url = base_url
  end

  def get(path) = request(Net::HTTP::Get, path)

  def put(path, body) = request(Net::HTTP::Put, path, body)

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
      return NotFound if response.is_a?(Net::HTTPNotFound)

      ERRORS_BY_CODE.fetch(error_code(response.body), Error)
    end

    def error_code(body)
      JSON.parse(body)["error"]
    rescue JSON::ParserError, TypeError
      nil
    end
end
