require "net/http"

# Conexão com a API REST do servidor markupp. Só o Note fala com ela.
class MarkuppApi
  class Error < StandardError; end

  NETWORK_ERRORS = [ SystemCallError, SocketError, Net::OpenTimeout, Net::ReadTimeout ].freeze

  def initialize(base_url)
    @base_url = base_url
  end

  def get(path)
    response = Net::HTTP.get_response(URI.join(@base_url, path))
    raise Error, "GET #{path} answered #{response.code}, expected 2xx: #{response.body}" unless response.is_a?(Net::HTTPSuccess)

    JSON.parse(response.body)
  rescue *NETWORK_ERRORS => error
    raise Error, "GET #{path} could not reach #{@base_url}: #{error.message}"
  end
end
