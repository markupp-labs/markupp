# Cofre publicado, lido sem conta pelas rotas /v1/public/vaults/{slug} (ADR-0048): sem id, sem tenant e sem quem
# publicou. Slug inexistente e cofre não publicado levantam o mesmo MarkuppApi::NotFound.
PublishedVault = Data.define(:slug, :title, :published_at, :graph_version) do
  # O máximo que a API devolve numa página.
  self::PAGE_LIMIT = 100

  def self.find(api, slug) = from_api(api.get(MarkuppApi.path("v1", "public", "vaults", slug)))

  def self.from_api(attributes)
    fields = attributes.symbolize_keys.slice(*members)
    new(**fields, published_at: Time.zone.parse(fields.fetch(:published_at)))
  end

  # Todas as notas do cofre, sem o conteúdo.
  def notes(api, page_limit: PublishedVault::PAGE_LIMIT) = Note.list_all_from(api, "public", "vaults", slug, "notes", page_limit:)

  # A nota com o conteúdo; nota de outro cofre levanta NotFound.
  def note(api, id) = Note.from_api(api.get(MarkuppApi.path("v1", "public", "vaults", slug, "notes", id)))

  def graph(api, store: Rails.cache) = Graph.of_published_vault(api, slug, store:)
end
