# Tenants da FakeMarkuppApi: vínculos conta-tenant-papel, /me/tenants, POST /tenants, convites recebidos em /me/invites
# e em /v1/me/notifications, e o tenant do cabeçalho, resolvido como o RequireTenant do servidor.
class FakeMarkuppTenants
  # Todos os convites, de todos os tenants: a FakeMarkuppTenantAdmin emite, lista e revoga por aqui.
  attr_reader :invites

  DEFAULT_TENANTS = [ { id: "tenant-1", name: "Acme research", members: { FakeMarkuppAccounts::DEFAULT_EMAIL => "owner" } } ].freeze
  NAME_LIMIT = 100

  INVITE_ANSWER = %r{\A/me/invites/(?<id>[^/]+)/(?<answer>accept|decline)\z}
  NOTIFICATIONS = "/v1/me/notifications".freeze

  # tenants: [{ id:, name:, members: { email => papel }, notes_count:, last_note_at: }]; as estatísticas são opcionais,
  # como no servidor que ainda não as manda, e só aparecem em /me/tenants quando o tenant as tem.
  # invites: [{ id:, tenant_id:, email:, role:, token:, expires_at:, accepted_at: }]; sem accepted_at e no prazo, pendente.
  def initialize(tenants, invites = [])
    @tenants = tenants.map do |tenant|
      { "id" => tenant[:id], "name" => tenant[:name], "members" => tenant[:members].dup, "stats" => tenant.slice(:notes_count, :last_note_at).stringify_keys }
    end
    @invites = invites.map { pending_defaults(it) }
    @notes_denied = []
  end

  def get(path, account:, **)
    return memberships(account) if path == "/me/tenants"
    return pending_invites(account) if path == "/me/invites"
    return notifications(account) if path == NOTIFICATIONS

    raise ArgumentError, "fake API has no route for GET #{path}, expected /me/tenants, /me/invites or #{NOTIFICATIONS}"
  end

  def post(path, body, account:, **)
    return create_tenant(body, account) if path == "/tenants"

    answer = INVITE_ANSWER.match(path) or raise ArgumentError, "fake API has no route for POST #{path}, expected /tenants or #{INVITE_ANSWER.source}"
    answer[:answer] == "accept" ? accept_invite(answer[:id], account) : decline_invite(answer[:id], account)
  end

  # Convite do POST /auth/invites/accept, ainda sem consumir: a conta pode já existir.
  def invite_for_token(token)
    open_invites = @invites.select { pending?(it) }
    open_invites.find { it["token"].present? && it["token"] == token } or
      raise MarkuppApi::InvalidToken, "fake API refuses invite token #{token.inspect}: invalid_invite, expected one of #{open_invites.filter_map { it["token"] }}"
  end

  # Consome o convite, que fica aceito na lista do admin, e cria o vínculo com o papel dele; devolve o tenant como /me/tenants.
  def join(invite, account)
    invite["accepted_at"] = Time.current.utc.iso8601
    tenant = tenant(invite["tenant_id"])
    tenant["members"][account["email"]] = invite["role"]
    membership(tenant, account)
  end

  # Vínculo da conta com o tenant do cabeçalho, { id, name, role }; sem cabeçalho vale o único tenant da conta.
  def membership_for(account, requested, call)
    raise MarkuppApi::TenantUnavailable, "fake API refuses #{call} in tenant #{requested.inspect}: tenant_forbidden" if @notes_denied.include?(requested)
    return requested_membership(account, requested, call) if requested.present?

    only_membership(account, call)
  end

  def tenant(id) = @tenants.find { it["id"] == id }

  # Como a API: convite sem resposta e no prazo.
  def pending?(invite) = invite["accepted_at"].nil? && Time.zone.parse(invite["expires_at"]).future?

  # Cenário: chega um convite com a página já aberta, como o de outro admin. invite como no initialize.
  def receive_invite!(invite) = @invites << pending_defaults(invite)

  # Cenário: o papel muda (ou some, com nil) no servidor entre duas páginas.
  def assign_role!(tenant_id, email, role)
    members = tenant(tenant_id).fetch("members")
    role ? members[email] = role : members.delete(email)
  end

  # Cenário: a conta perde o tenant entre o GET /me/tenants e a chamada de notas da mesma página.
  def deny_notes_in!(tenant_id) = @notes_denied << tenant_id

  # Tenant apagado pelo owner (FakeMarkuppTenantDeletion), ou por outra sessão no meio do cenário: some com os convites.
  def delete_tenant!(id)
    @tenants.reject! { it["id"] == id }
    @invites.reject! { it["tenant_id"] == id }
  end

  private
    # Como a API: nome sem espaços nas pontas, de 1 a 100 bytes; quem cria é owner. Os ids seguem a ordem: tenant-2...
    def create_tenant(body, account)
      name = body[:name].to_s.strip
      raise MarkuppApi::InvalidRequest, "fake API refuses POST /tenants with name #{body[:name].inspect}, expected 1 to #{NAME_LIMIT} bytes" unless name.bytesize.between?(1, NAME_LIMIT)

      @tenants << { "id" => "tenant-#{@tenants.size + 1}", "name" => name, "members" => { account["email"] => "owner" } }
      membership(@tenants.last, account)
    end

    # Como a API: só a conta de email verificado vê os convites endereçados a ela.
    def pending_invites(account)
      return [] unless account.fetch("verified", true)

      @invites.select { it["email"] == account["email"] && pending?(it) }.map { pending_invite(it) }
    end

    def pending_defaults(invite)
      invite.stringify_keys.reverse_merge("expires_at" => 7.days.from_now.utc.iso8601, "created_at" => Time.current.utc.iso8601)
    end

    # Como a API (ADR-0049): os convites pendentes, cada um com o kind, e quantos pedem resposta.
    def notifications(account)
      items = pending_invites(account).map { { "kind" => "invite", **it } }
      { "items" => items, "unread" => items.size }
    end

    def pending_invite(invite)
      tenant = tenant(invite["tenant_id"])
      { "id" => invite["id"], "tenant_id" => tenant["id"], "tenant_name" => tenant["name"], "role" => invite["role"], "expires_at" => invite["expires_at"] }
    end

    def accept_invite(id, account)
      invite = invite_of(id, account)
      raise MarkuppApi::AlreadyMember, "fake API refuses to accept #{id.inspect}: #{account["email"]} is already in #{invite["tenant_id"]}" if memberships(account).any? { it["id"] == invite["tenant_id"] }

      join(invite, account)
    end

    def decline_invite(id, account)
      @invites.delete(invite_of(id, account))
      nil
    end

    # Como a API, convite de outra conta é igual a convite inexistente.
    def invite_of(id, account)
      pending_invites(account).any? { it["id"] == id } or
        raise MarkuppApi::InviteNotFound, "fake API has no pending invite #{id.inspect} for #{account["email"]}, expected one of #{pending_invites(account).map { it["id"] }}"

      @invites.find { it["id"] == id }
    end

    def memberships(account)
      @tenants.select { it["members"].key?(account["email"]) }.map { membership(it, account) }
    end

    def membership(tenant, account)
      { "id" => tenant["id"], "name" => tenant["name"], "role" => tenant["members"].fetch(account["email"]), **tenant.fetch("stats", {}) }
    end

    def requested_membership(account, requested, call)
      memberships(account).find { it["id"] == requested } or
        raise MarkuppApi::TenantUnavailable, "fake API refuses #{call} in tenant #{requested.inspect}: tenant_forbidden, expected one of #{memberships(account).map { it["id"] }}"
    end

    def only_membership(account, call)
      mine = memberships(account)
      raise MarkuppApi::TenantUnavailable, "fake API refuses #{call} without X-Markupp-Tenant: no_tenant" if mine.empty?
      raise MarkuppApi::TenantUnavailable, "fake API refuses #{call} without X-Markupp-Tenant: tenant_required among #{mine.map { it["id"] }}" if mine.many?

      mine.sole
    end
end
