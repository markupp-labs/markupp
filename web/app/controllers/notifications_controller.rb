# Sino da barra de atividades (ADR-0049): a mesma resposta traz os contadores das duas barras (a coluna do desktop e a
# linha da gaveta) e a lista do popover, e o Turbo fica com o frame que pediu. Sem frame é a página da lista.
class NotificationsController < ApplicationController
  def show
    @feed = AccountNotification.feed(credential.api)
  end

  private
    # O contador fica dentro do botão do sino: o aviso (sessão vencida, tenant que sumiu) vai para a lista, e o
    # contador volta sem número, sem link dentro do botão.
    def render_frame_alert(path, alert, status:)
      render :unavailable, locals: { path:, alert: }, status:
    end
end
