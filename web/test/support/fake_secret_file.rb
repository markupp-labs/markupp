# Faz o papel do arquivo onde o Go escreve o segredo do MCP: ausente até write, e o conteúdo dado depois.
class FakeSecretFile
  def initialize(content = nil) = @content = content

  def write(content) = @content = content

  def read(path)
    raise Errno::ENOENT, path if @content.nil?

    @content
  end
end
