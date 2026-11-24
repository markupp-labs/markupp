# Cadastro por método da FakeMarkuppApi sob /v1/instance/registration-methods, como a API (ADR-0059): só a conta com
# instance_admin lê e troca, o método travado por quem hospeda recusa o PATCH e o desconhecido não existe. methods é a
# lista como a API devolve, com id, name, open e locked.
class FakeMarkuppInstanceRegistration
  include FakeInstanceAdminGuard

  ROUTE = "/v1/instance/registration-methods".freeze
  DEFAULT_METHODS = [
    { "id" => "local", "name" => "Email and password", "open" => true, "locked" => false },
    { "id" => "github", "name" => "GitHub", "open" => true, "locked" => false }
  ].freeze

  attr_reader :updates

  def initialize(methods)
    @methods = methods.map(&:dup)
    @updates = []
  end

  def get(path, account:, **)
    refuse_non_admin(account, "GET #{path}")
    @methods
  end

  def patch(path, body, account:, **)
    call = "PATCH #{path}"
    refuse_non_admin(account, call)
    method = @methods.find { path == "#{ROUTE}/#{it["id"]}" }
    raise MarkuppApi::NotFound, "fake API has no registration method at #{call}" unless method
    raise MarkuppApi::HostLocked, "fake API refuses #{call}: host_locked, expected an unlocked method" if method["locked"]

    @updates << { id: method["id"], open: body[:open] }
    method.merge!("open" => body[:open])
  end
end
