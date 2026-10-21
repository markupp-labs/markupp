# Pedidos OAuth da FakeMarkuppApi: GET /v1/oauth/requests/{id} e as decisões approve e deny, como o authorization
# server. Pedido inexistente, vencido ou já decidido é o mesmo 404, como na API.
class FakeMarkuppOAuthRequests
  REQUEST = %r{\A/v1/oauth/requests/(?<id>[^/]+)(?:/(?<decision>approve|deny))?\z}
  FIELDS = %w[id client_id client_name client_source redirect_host scopes already_authorized expires_at].freeze

  # [[id, decisão, email da conta]], na ordem em que chegaram.
  attr_reader :decisions

  # requests: [{ id:, client_name:, redirect_uri:, scopes:, already_authorized:, expires_at: }]; o resto ganha um padrão.
  # A aprovação devolve o redirect_uri com o código, e a recusa com access_denied, como o servidor.
  def initialize(requests)
    @requests = requests.map { default_request(it[:id]).merge(it.stringify_keys) }
    @decisions = []
  end

  def get(path, account:, **)
    match = route(path, "GET")
    pending_request(match[:id], account).slice(*FIELDS)
  end

  def post(path, _body, account:, **)
    match = route(path, "POST")
    request = pending_request(match[:id], account)
    @requests.delete(request)
    @decisions << [ request["id"], match[:decision], account["email"] ]
    { "redirect_to" => redirect_for(request, match[:decision]) }
  end

  private
    def default_request(id)
      { "client_id" => "https://claude.ai/oauth/client", "client_name" => "Claude", "client_source" => "cimd",
        "redirect_uri" => "https://claude.ai/api/callback", "redirect_host" => "claude.ai",
        "scopes" => %w[notas:leitura], "already_authorized" => false, "expires_at" => 10.minutes.from_now.utc.iso8601, "id" => id }
    end

    def route(path, verb)
      match = REQUEST.match(path)
      return match if match && match[:decision].nil? == (verb == "GET")

      raise ArgumentError, "fake API has no route for #{verb} #{path}, expected #{REQUEST.source}"
    end

    # Como a API, só a conta logada vê e decide o pedido.
    def pending_request(id, account)
      raise MarkuppApi::Unauthorized, "fake API refuses the oauth request #{id.inspect} without a login token" unless account

      @requests.find { it["id"] == id && Time.zone.parse(it["expires_at"]).future? } or
        raise MarkuppApi::OAuthRequestNotFound, "fake API has no pending oauth request #{id.inspect}: request_not_found, expected one of #{@requests.map { it["id"] }}"
    end

    def redirect_for(request, decision)
      outcome = decision == "approve" ? "code=code-#{request["id"]}" : "error=access_denied"
      "#{request["redirect_uri"]}?#{outcome}&state=state-#{request["id"]}"
    end
end
