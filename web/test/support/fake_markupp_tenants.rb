# Tenants da FakeMarkuppApi: vínculos conta-tenant-papel, /me/tenants, POST /tenants e o tenant do cabeçalho,
# resolvido como o RequireTenant do servidor.
class FakeMarkuppTenants
  DEFAULT_TENANTS = [ { id: "tenant-1", name: "Acme research", members: { FakeMarkuppAccounts::DEFAULT_EMAIL => "owner" } } ].freeze
  NAME_LIMIT = 100

  # tenants: [{ id:, name:, members: { email => papel } }]
  def initialize(tenants)
    @tenants = tenants.map { |tenant| { "id" => tenant[:id], "name" => tenant[:name], "members" => tenant[:members].dup } }
    @notes_denied = []
  end

  def get(path, account:, **)
    raise ArgumentError, "fake API has no route for GET #{path}, expected /me/tenants" unless path == "/me/tenants"

    memberships(account)
  end

  # Como a API: nome sem espaços nas pontas, de 1 a 100 bytes; quem cria é owner. Os ids seguem a ordem: tenant-2...
  def post(path, body, account:, **)
    raise ArgumentError, "fake API has no route for POST #{path}, expected /tenants" unless path == "/tenants"

    name = body[:name].to_s.strip
    raise MarkuppApi::InvalidRequest, "fake API refuses POST /tenants with name #{body[:name].inspect}, expected 1 to #{NAME_LIMIT} bytes" unless name.bytesize.between?(1, NAME_LIMIT)

    @tenants << { "id" => "tenant-#{@tenants.size + 1}", "name" => name, "members" => { account["email"] => "owner" } }
    membership(@tenants.last, account)
  end

  # Papel da conta no tenant do cabeçalho; sem cabeçalho vale o único tenant da conta.
  def role_for(account, requested, call)
    raise MarkuppApi::TenantUnavailable, "fake API refuses #{call} in tenant #{requested.inspect}: tenant_forbidden" if @notes_denied.include?(requested)
    return requested_membership(account, requested, call)["role"] if requested.present?

    only_membership(account, call)["role"]
  end

  # Cenário: o papel muda (ou some, com nil) no servidor entre duas páginas.
  def assign_role!(tenant_id, email, role)
    members = @tenants.find { it["id"] == tenant_id }.fetch("members")
    role ? members[email] = role : members.delete(email)
  end

  # Cenário: a conta perde o tenant entre o GET /me/tenants e a chamada de notas da mesma página.
  def deny_notes_in!(tenant_id) = @notes_denied << tenant_id

  private
    def memberships(account)
      @tenants.select { it["members"].key?(account["email"]) }.map { membership(it, account) }
    end

    def membership(tenant, account) = { "id" => tenant["id"], "name" => tenant["name"], "role" => tenant["members"].fetch(account["email"]) }

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
