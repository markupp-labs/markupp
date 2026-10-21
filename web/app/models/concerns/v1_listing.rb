# Lista de um recurso sob /v1 da API, cada item montado pelo from_api de quem estende.
module V1Listing
  def list_from(api, *segments, **query) = api.get(MarkuppApi.path("v1", *segments, **query)).map { from_api(it) }
end
