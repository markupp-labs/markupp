class SearchesController < ApplicationController
  # Busca vazia não chama a API, nem para saber o tenant.
  before_action :require_tenant, if: -> { params[:q].present? }

  def show
    @notes = Note.search(credential.api, params[:q])
  end
end
