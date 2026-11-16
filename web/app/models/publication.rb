# Publicação do tenant (ADR-0048), como /v1/publications devolve: uma nota, uma pasta (com as notas que entrarem nela)
# ou o cofre inteiro, aberto à leitura sem conta. Quem pode publicar o quê e o nome que confirma ficam na API; o painel
# pergunta o mesmo nome antes, para ninguém publicar por engano.
Publication = Data.define(:id, :vault_id, :vault_name, :kind, :note_id, :folder_path, :target_name, :note_count, :indexable,
                          :published_by, :published_at, :public_id, :title_slug) do
  extend TimestampedFromApi

  # O que fica público, antes de publicar: quantas notas e se as que vierem depois entram.
  def self.preview(api, vault_id, target)
    attributes = api.get(MarkuppApi.path("v1", "vaults", vault_id, "publications", "preview", **target.to_api))
    Publication::Preview.new(**attributes.symbolize_keys.slice(*Publication::Preview.members))
  end

  # confirm é o nome do alvo digitado; outro nome levanta ConfirmationMismatch.
  def self.publish(api, vault_id, target, confirm:, indexable:)
    from_api(api.post(MarkuppApi.path("v1", "vaults", vault_id, "publications"), target: target.to_api, confirm:, indexable:))
  end

  # As do cofre, ou as de todos os cofres do tenant sem ele; a API só lista as que a conta vê.
  def self.all(api, vault_id: nil) = api.get(MarkuppApi.path("v1", "publications", vault_id:)).fetch("items").map { from_api(it) }

  def self.change_indexing(api, id, indexable) = from_api(api.patch(publication_path(id), indexable:))

  def self.unpublish(api, id) = api.delete(publication_path(id))

  def self.publication_path(id) = MarkuppApi.path("v1", "publications", id)

  def self.from_api(attributes) = timestamped_from(members.to_h { [ it.to_s, nil ] }.merge(attributes), :published_at)

  private_class_method :publication_path

  def published_by_name = published_by&.fetch("name", nil)

  # O fim do endereço público da nota publicada sozinha; pasta e cofre não têm um endereço só.
  def public_param = ("#{title_slug}-#{public_id}" if public_id)
end

# A prévia de GET /v1/vaults/{id}/publications/preview.
Publication::Preview = Data.define(:kind, :target_name, :note_count, :includes_future)

# O alvo de uma publicação: a nota pelo id, a pasta pelo caminho ou o cofre, que é a pasta raiz.
Publication::Target = Data.define(:kind, :note_id, :path) do
  def initialize(kind:, note_id: nil, path: nil) = super

  # O alvo vindo de um pedido do painel; tipo desconhecido não chega à API.
  def self.from(kind:, note_id: nil, path: nil)
    return new(kind:, note_id:, path:) if Publication::Target::KINDS.include?(kind)

    raise MarkuppApi::InvalidRequest, "publication kind #{kind.inspect} is unknown, expected note, folder or vault"
  end

  def to_api = { kind:, note_id:, path: }.compact
end

Publication::Target::KINDS = %w[note folder vault].freeze
