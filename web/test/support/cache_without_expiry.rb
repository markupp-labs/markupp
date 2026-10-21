# Cache que ignora expires_in: deixa o teste ver a expiração embutida na mensagem, sem o TTL do cache esconder. Guarda o
# expires_in de cada escrita, pela chave, para o teste conferir quanto tempo a entrada viveria num cache de verdade.
class CacheWithoutExpiry
  attr_reader :expiries

  def initialize
    @entries = {}
    @expiries = {}
  end

  def read(key, **) = @entries[key]

  def write(key, value, expires_in: nil, **)
    @expiries[key] = expires_in
    @entries[key] = value
  end
end
