# Conta da instância como o administrador a vê em GET /v1/instance/accounts (ADR-0061): como entra, quando foi o último
# acesso (nil sem nenhum) e se está bloqueada. O detalhe traz os tenants que somem com ela.
InstanceAccount = Data.define(:id, :email, :email_verified, :login_methods, :created_at, :last_access_at, :blocked) do
  self::PAGE_SIZE = 50
  self::STATUSES = %w[active blocked].freeze

  # Detalhe: a conta e os tenants em que ela é a única owner, cada um com quantas notas tem.
  self::Detail = Data.define(:account, :sole_owner_tenants) do
    def notes = sole_owner_tenants.sum { it.fetch("notes") }
  end

  def self.page(api, q: nil, status: nil, offset: 0)
    query = { q: q.presence, status: status.presence_in(self::STATUSES), limit: self::PAGE_SIZE, offset: }
    api.get(path(**query)).map { from_api(it) }
  end

  def self.find(api, id)
    attributes = api.get(path(id))
    self::Detail.new(account: from_api(attributes), sole_owner_tenants: attributes.fetch("sole_owner_tenants"))
  end

  def self.set_blocked(api, id, blocked) = api.patch(path(id), { blocked: })

  def self.destroy(api, id) = api.delete(path(id))

  def self.sessions(api, id) = api.get(path(id, "sessions")).map { AccountSession.from_api(it) }

  def self.end_session(api, id, session_id) = api.delete(path(id, "sessions", session_id))

  def self.authorizations(api, id) = api.get(path(id, "authorizations")).map { OAuthGrant.from_api(it) }

  def self.revoke_authorization(api, id, authorization_id) = api.delete(path(id, "authorizations", authorization_id))

  def self.path(*segments, **query) = MarkuppApi.path("v1", "instance", "accounts", *segments, **query)

  def self.from_api(attributes)
    fields = attributes.symbolize_keys.slice(*members)
    last = fields[:last_access_at]
    new(**fields, created_at: Time.zone.parse(fields.fetch(:created_at)), last_access_at: last && Time.zone.parse(last))
  end
end
