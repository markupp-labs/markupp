class SearchesController < ApplicationController
  RESULT_LIMIT = 20

  # Busca vazia não chama a API, nem para saber o tenant.
  before_action :require_tenant, if: -> { params[:q].present? }

  # Busca ranqueada em todos os cofres do tenant, do mais relevante ao menos, com o trecho de cada nota.
  def show
    @hits = SearchHit.search(credential.api, params[:q], limit: RESULT_LIMIT)
  end
end
