# Cache que ignora expires_in: deixa o teste ver a expiração embutida na mensagem, sem o TTL do cache esconder.
class CacheWithoutExpiry
  def initialize = @entries = {}

  def read(key, **) = @entries[key]

  def write(key, value, **) = @entries[key] = value

  def fetch(key, **)
    @entries.fetch(key) { @entries[key] = yield }
  end
end
