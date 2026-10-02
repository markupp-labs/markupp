require "socket"

# Servidor HTTP de uma resposta só, em porta efêmera, para testar o MarkuppApi.
class FakeHttpServer
  attr_reader :requested_path

  def initialize(status:, body:)
    @status = status
    @body = body
    @server = TCPServer.new("127.0.0.1", 0)
  end

  def url = "http://127.0.0.1:#{@server.addr[1]}"

  def serve_once
    Thread.new do
      client = @server.accept
      @requested_path = client.gets.split[1]
      client.write("HTTP/1.1 #{@status}\r\nContent-Type: application/json\r\n" \
                   "Content-Length: #{@body.bytesize}\r\nConnection: close\r\n\r\n#{@body}")
      client.close
    end
  end

  def close = @server.close
end
