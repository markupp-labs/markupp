# Lista de um recurso sob /v1 da API, cada item montado pelo from_api de quem estende.
module V1Listing
  # O maior offset que a API aceita (maxOffset em markupp/internal/api/notes_handler.go); acima dele ela corta para o
  # teto sem erro e repete a mesma página cheia.
  MAX_OFFSET = 10_000

  def list_from(api, *segments, **query) = api.get(MarkuppApi.path("v1", *segments, **query)).map { from_api(it) }

  # A lista inteira, de página em página de page_limit itens, até a primeira página que vem menor ou até a página no
  # teto de offset da API: a lista maior que isso vem truncada ali, porque a API não pagina além (não tem cursor).
  # ponytail: uma chamada por página; lista de milhares de itens pede paginar quem mostra.
  def list_all_from(api, *segments, page_limit:, **query)
    pages = (0..V1Listing::MAX_OFFSET).step(page_limit).lazy.map { list_from(api, *segments, **query, offset: it, limit: page_limit) }
    pages.slice_after { it.size < page_limit }.first.flatten
  end
end
