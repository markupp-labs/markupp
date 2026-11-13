# Nota pública lida sem conta pelas rotas /v1/public/notes/{public_id} (ADR-0048): sem id interno, sem tenant e sem quem
# publicou. Nota não publicada e id inexistente levantam o mesmo MarkuppApi::NotFound. A árvore, a busca e o grafo
# são os da publicação mais ampla que cobre a nota.
PublicNote = Data.define(:public_id, :title, :title_slug, :content, :updated_at, :indexable, :publication, :links) do
  # O id público tem 12 caracteres alfanuméricos.
  self::ID_FORMAT = /\A[A-Za-z0-9]{12}\z/
  # O máximo que a API devolve numa página.
  self::PAGE_LIMIT = 100
  # A árvore guardada vale pouco: a lista não tem versão própria na API, e nota nova ou despublicada aparece em até isso.
  self::STORED_FOR = 1.minute

  # O slug e o id de /p/{slug}-{id}: o id é o trecho depois do último hífen. Fora do formato nem chega à API.
  def self.id_in(slug_and_id)
    slug, _, id = slug_and_id.to_s.rpartition("-")
    return [ slug, id ] if PublicNote::ID_FORMAT.match?(id)

    raise MarkuppApi::NotFound, "public address #{slug_and_id.inspect} has no public id, expected {title_slug}-{12 letters or digits}"
  end

  def self.find(api, public_id) = from_api(api.get(MarkuppApi.path("v1", "public", "notes", public_id)))

  def self.from_api(attributes)
    fields = attributes.symbolize_keys.slice(*members)
    new(**fields, publication: PublicNote::Publication.new(**fields.fetch(:publication).symbolize_keys.slice(*PublicNote::Publication.members)),
                  links: fields.fetch(:links, []).map { PublicNote::Link.new(**it.symbolize_keys.slice(*PublicNote::Link.members)) })
  end

  def to_param = "#{title_slug}-#{public_id}"

  # As ligações para notas públicas; as outras a leitura mostra como texto.
  def public_links = links.select(&:public_id)

  # As notas da publicação como Note (id e public_id são o id público), com o caminho dentro da pasta publicada.
  # ponytail: uma chamada por página; a lista para no teto de offset da API (V1Listing::MAX_OFFSET).
  def tree(api, store: Rails.cache)
    store.fetch("panel/public/tree/#{public_id}", expires_in: PublicNote::STORED_FOR) do
      V1Listing.every_page(PublicNote::PAGE_LIMIT) { |offset, limit| api.get(listing_path("tree", offset:, limit:)).fetch("items") }
    end.map { Note.new(id: it["public_id"], **it.slice("public_id", "title_slug", "path")) }
  end

  # A primeira página da busca, só na publicação.
  def search(api, query) = api.get(listing_path("search", q: query)).fetch("items").map { PublicNote::Hit.new(**it.symbolize_keys.slice(*PublicNote::Hit.members)) }

  def graph(api, store: Rails.cache) = Graph.of_public_note(api, public_id, store:)

  private
    def listing_path(resource, **query) = MarkuppApi.path("v1", "public", "notes", public_id, resource, **query)
end

# A publicação que dá o recorte: o tipo, o título (da nota, da pasta ou do cofre) e a nota por onde ela começa.
PublicNote::Publication = Data.define(:kind, :title, :root_public_id)

# Uma ligação da nota: public_id nil quando o alvo não é público.
PublicNote::Link = Data.define(:cited_title, :kind, :public_id, :title_slug)

# Um resultado da busca pública, com o trecho marcado entre U+0002 e U+0003.
PublicNote::Hit = Data.define(:public_id, :title, :title_slug, :snippet) do
  def to_param = "#{title_slug}-#{public_id}"
end
