# exceptions_app do painel (config/application.rb): a página de erro sai das rotas, pelo ErrorsController.
class PanelExceptionsApp
  def initialize(panel:, static:, logger:)
    @panel = panel
    @static = static
    @logger = logger
  end

  def call(env) = @panel.call(env)
end
