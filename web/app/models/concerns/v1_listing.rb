# Lista de um recurso sob /v1 da API, cada item montado pelo from_api de quem estende.
module V1Listing
  # O maior offset que a API aceita; acima dele ela corta para o teto sem erro e repete a mesma página cheia.
  MAX_OFFSET = 10_000

  def list_from(api, *segments, **query) = api.get(MarkuppApi.path("v1", *segments, **query)).map { from_api(it) }

  # Os itens de todas as páginas de page_limit, que o bloco lê a partir de cada offset, até a primeira página menor ou
  # até o teto de offset da API: lista maior que isso vem truncada ali, porque a API não pagina além.
  def self.every_page(page_limit)
    pages = (0..MAX_OFFSET).step(page_limit).lazy.map { yield it, page_limit }
    pages.slice_after { it.size < page_limit }.first.flatten
  end
end
