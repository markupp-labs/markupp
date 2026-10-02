require "net/http"

# Conexão com a API REST do servidor markupp. Só o Note fala com ela.
class MarkuppApi
  class Error < StandardError; end
  class NotFound < Error; end
  class Conflict < Error; end
  class DuplicatePath < Error; end

  NETWORK_ERRORS = [ SystemCallError, SocketError, Net::OpenTimeout, Net::ReadTimeout ].freeze
  # A API responde 409 por dois motivos, distinguidos pelo campo error do corpo.
  CONFLICT_ERRORS = { "conflict" => Conflict, "duplicate_path" => DuplicatePath }.freeze

  def initialize(base_url)
    @base_url = base_url
  end

  def get(path) = request(Net::HTTP::Get, path)

  def put(path, body) = request(Net::HTTP::Put, path, body)

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

    def parse_response(response, call)
      return JSON.parse(response.body) if response.is_a?(Net::HTTPSuccess)

      raise error_class_for(response), "#{call} answered #{response.code}, expected 2xx: #{response.body}"
    end

    def error_class_for(response)
      return NotFound if response.is_a?(Net::HTTPNotFound)
      return Error unless response.is_a?(Net::HTTPConflict)

      CONFLICT_ERRORS.fetch(error_code(response.body), Error)
    end

    def error_code(body)
      JSON.parse(body)["error"]
    rescue JSON::ParserError, TypeError
      nil
    end
end
