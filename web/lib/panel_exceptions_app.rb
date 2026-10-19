# exceptions_app do painel (config/application.rb): a página de erro sai das rotas, pelo ErrorsController. Se ela mesma
# quebrar, o ShowExceptions só daria um texto puro; a estática de public/, que não depende de nada do app, entra no lugar.
class PanelExceptionsApp
  def initialize(panel:, static:, logger:)
    @panel = panel
    @static = static
    @logger = logger
  end

  def call(env)
    @panel.call(env)
  rescue StandardError => error
    @logger.error({ event: "panel_error_page_failed", path: env["PATH_INFO"], message: error.message }.to_json)
    @static.call(env)
  end
end
