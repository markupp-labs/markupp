require "socket"

# Servidor HTTP de uma resposta só, em porta efêmera, para testar o MarkuppApi.
class FakeHttpServer
  attr_reader :requested_method, :requested_path, :requested_body, :requested_headers

  # headers vão na resposta além de Content-Type, Content-Length e Connection.
  def initialize(status:, body:, headers: {})
    @status = status
    @body = body
    @headers = headers
    @server = TCPServer.new("127.0.0.1", 0)
  end

  def url = "http://127.0.0.1:#{@server.addr[1]}"

  def serve_once
    Thread.new do
      client = @server.accept
      read_request(client)
      extra = @headers.map { |name, value| "#{name}: #{value}\r\n" }.join
      client.write("HTTP/1.1 #{@status}\r\nContent-Type: application/json\r\n#{extra}" \
                   "Content-Length: #{@body.bytesize}\r\nConnection: close\r\n\r\n#{@body}")
      client.close
    end
  end

  # Lê o pedido e segura a conexão sem responder, para o cliente esgotar o read_timeout.
  def hold_once(seconds)
    Thread.new do
      client = @server.accept
      read_request(client)
      sleep seconds
      client.close
    end
  end

  # Conexão esperando no backlog sem ser aceita: o cliente tentou de novo.
  def pending_connection?
    @server.accept_nonblock.close
    true
  rescue IO::WaitReadable
    false
  end

  def close = @server.close

  private
    def read_request(client)
      @requested_method, @requested_path = client.gets.split
      @requested_headers = client.each_line.take_while { |line| line != "\r\n" }.to_h { |line| header_pair(line) }
      @requested_body = client.read(@requested_headers["content-length"].to_i)
    end

    # Nome em minúsculas, como o HTTP trata.
    def header_pair(line)
      name, value = line.split(":", 2)
      [ name.downcase, value.strip ]
    end
end
