# Lista de um recurso sob /v1 da API, cada item montado pelo from_api de quem estende.
module V1Listing
  def list_from(api, *segments, **query) = api.get(MarkuppApi.path("v1", *segments, **query)).map { from_api(it) }

  # A lista inteira, de página em página de page_limit itens, até a primeira página que vem menor.
  # ponytail: uma chamada por página, sem teto; lista de milhares de itens pede paginar quem mostra.
  def list_all_from(api, *segments, page_limit:, **query)
    pages = (0..).lazy.map { list_from(api, *segments, **query, offset: it * page_limit, limit: page_limit) }
    pages.slice_after { it.size < page_limit }.first.flatten
  end
end
