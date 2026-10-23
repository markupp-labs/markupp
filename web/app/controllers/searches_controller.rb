class SearchesController < ApplicationController
  RESULT_LIMIT = 20

  # Busca vazia não chama a API, nem para saber o tenant.
  before_action :require_tenant, if: -> { params[:q].present? }

  # Busca ranqueada no cofre ativo, do mais relevante ao menos, com o trecho de cada nota; sem cofre, nada.
  def show
    vault = reachable_vault if params[:q].present?
    @hits = vault ? SearchHit.search(credential.api, params[:q], vault: vault.id, limit: RESULT_LIMIT) : []
  end
end
