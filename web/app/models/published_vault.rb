# Cofre publicado, lido sem conta pelas rotas /v1/public/vaults/{slug} (ADR-0048): sem id, sem tenant e sem quem
# publicou. Slug inexistente e cofre não publicado levantam o mesmo MarkuppApi::NotFound.
# O cofre e a lista das notas ficam guardados em store por PublishedVault::STORED_FOR (a lista, também pela versão do
# grafo): cada visita e cada ![[nota]] lê só a nota na API, em vez de refazer o cofre e todas as páginas da lista. A
# despublicação e a nota nova aparecem na árvore em até esse tempo; a nota em si é lida a cada pedido.
PublishedVault = Data.define(:slug, :title, :published_at, :graph_version) do
  # O máximo que a API devolve numa página.
  self::PAGE_LIMIT = 100

  def self.find(api, slug, store: Rails.cache)
    from_api(kept(store, "panel/public/#{slug}") { api.get(MarkuppApi.path("v1", "public", "vaults", slug)) })
  end

  # O guardado em key, ou o que o bloco lê, guardado por PublishedVault::STORED_FOR; erro da API não fica guardado.
  def self.kept(store, key)
    stored = store.read(key)
    return stored unless stored.nil?

    yield.tap { store.write(key, it, expires_in: PublishedVault::STORED_FOR) }
  end

  def self.from_api(attributes)
    fields = attributes.symbolize_keys.slice(*members)
    new(**fields, published_at: Time.zone.parse(fields.fetch(:published_at)))
  end

  # Todas as notas do cofre, sem o conteúdo.
  def notes(api, store: Rails.cache, page_limit: PublishedVault::PAGE_LIMIT)
    PublishedVault.kept(store, "panel/public/#{slug}/notes/#{graph_version}") do
      Note.list_all_from(api, "public", "vaults", slug, "notes", page_limit:)
    end
  end

  # A nota com o conteúdo; nota de outro cofre levanta NotFound.
  def note(api, id) = Note.from_api(api.get(MarkuppApi.path("v1", "public", "vaults", slug, "notes", id)))
end

# Quanto tempo o cofre e a lista das notas valem guardados: curto, porque a lista não tem versão própria na API.
PublishedVault::STORED_FOR = 1.minute
