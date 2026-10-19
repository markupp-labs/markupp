# Páginas de erro do painel, no layout público. O ShowExceptions pede /<status> (config.exceptions_app em application.rb)
# e o endereço que não casou com nenhuma rota chega como 404 pela rota coringa do fim do routes.rb.
class ErrorsController < ApplicationController
  skip_before_action :require_login
  # O erro pode vir de um POST (o 422 do token vencido): a página em si não muda nada.
  skip_forgery_protection

  layout "public"
  helper_method :way_back

  def show
    @code = params[:code].to_i
    return head(@code) if request.format.json?

    @asked_path = request.get_header("action_dispatch.original_path") || request.path
    render :show, status: @code, formats: :html
  end

  private
    # Com sessão volta às notas (a raiz abre a nota padrão); sem ela, ao login.
    def way_back
      credential.signed_in? ? [ t("errors.show.back_to_notes"), root_path ] : [ t("errors.show.back_to_login"), new_session_path ]
    end
end
