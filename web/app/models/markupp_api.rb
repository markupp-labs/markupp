require "net/http"

# Conexão com a API REST do servidor markupp. Só o Note fala com ela.
class MarkuppApi
  class Error < StandardError; end
  class NotFound < Error; end

  NETWORK_ERRORS = [ SystemCallError, SocketError, Net::OpenTimeout, Net::ReadTimeout ].freeze

  def initialize(base_url)
    @base_url = base_url
  end

  def get(path) = request(Net::HTTP::Get, path)

  private
    def request(verb, path)
      uri = URI.join(@base_url, path)
      response = Net::HTTP.start(uri.host, uri.port, use_ssl: uri.scheme == "https") { |http| http.request(verb.new(uri)) }
      parse_response(response, "#{verb::METHOD} #{path}")
    rescue *NETWORK_ERRORS => error
      raise Error, "#{verb::METHOD} #{path} could not reach #{@base_url}: #{error.message}"
    end

    def parse_response(response, call)
      return JSON.parse(response.body) if response.is_a?(Net::HTTPSuccess)

      error_class = response.is_a?(Net::HTTPNotFound) ? NotFound : Error
      raise error_class, "#{call} answered #{response.code}, expected 2xx: #{response.body}"
    end
end
