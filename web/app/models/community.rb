# Comunidade: o agrupamento das notas mais ligadas entre si do que com o resto do cofre, com o tamanho e a nota de
# maior PageRank (hub). As notas dela vêm de Note.in_vault com community: id.
Community = Data.define(:id, :size, :hub) do
  # Das maiores para as menores.
  def self.in_vault(api, vault_id, **query) = []
end
