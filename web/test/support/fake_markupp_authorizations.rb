# Autorizações da FakeMarkuppApi: GET /v1/me/authorizations e DELETE /v1/me/authorizations/{id}, como o authorization
# server. Cada conta vê e revoga só as suas; inexistente, já revogada ou de outra conta é o mesmo 404, como na API.
class FakeMarkuppAuthorizations
  ROUTE = %r{\A/v1/me/authorizations(?:/(?<id>[^/]+))?\z}
  FIELDS = %w[id client_id client_name scopes created_at updated_at].freeze

  # authorizations: [{ id:, account:, client_name:, scopes:, created_at: }], na ordem da API; account é o email da
  # conta dona, e o resto ganha um padrão.
  def initialize(authorizations)
    @authorizations = authorizations.map { default_authorization(it[:id]).merge(it.stringify_keys) }
  end

  def get(path, account:, **)
    route(path, "GET")
    @authorizations.select { it["account"] == account_email(account, path) }.map { it.slice(*FIELDS) }
  end

  def delete(path, account:, **)
    id = route(path, "DELETE")[:id]
    @authorizations.delete(owned_authorization(id, account_email(account, path)))
    nil
  end

  private
    def default_authorization(id)
      { "id" => id, "account" => FakeMarkuppAccounts::DEFAULT_EMAIL, "client_id" => "client-#{id}", "client_name" => "markupp-mcp (terminal)",
        "scopes" => %w[notas:leitura], "created_at" => "2026-10-16T12:00:00Z", "updated_at" => "2026-10-17T12:00:00Z" }
    end

    def route(path, verb)
      match = ROUTE.match(path)
      return match if match && match[:id].nil? == (verb == "GET")

      raise ArgumentError, "fake API has no route for #{verb} #{path}, expected #{ROUTE.source}"
    end

    def account_email(account, path)
      raise MarkuppApi::Unauthorized, "fake API refuses #{path} without a login token" unless account

      account["email"]
    end

    def owned_authorization(id, email)
      @authorizations.find { it["id"] == id && it["account"] == email } or
        raise MarkuppApi::AuthorizationNotFound,
              "fake API has no authorization #{id.inspect} for #{email}: authorization_not_found, expected one of #{@authorizations.map { it["id"] }}"
    end
end
