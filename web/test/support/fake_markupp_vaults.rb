# Cofres da FakeMarkuppApi sob /v1, no tenant do cabeçalho: /v1/vaults, os membros de cada cofre e os colegas do tenant
# em /v1/users, com as regras da API (só membro do cofre o vê, viewer não escreve mas sai sozinho, membro vem do tenant).
class FakeMarkuppVaults
  ROUTE = %r{\A/v1/(?:users|vaults(?:/(?<id>[^/]+)(?<members>/members(?:/(?<user_id>[^/]+))?)?)?)\z}
  NAME_LIMIT = 100
  CREATED_AT = "2026-09-01T12:00:00Z"
  # Cofre de cada tenant quando o cenário não diz: o vault-1 das notas da FakeMarkuppNotes, com todo membro do tenant.
  DEFAULT_NAME = "Notes"

  # vaults: [{ id:, tenant_id:, name:, members: [email] }]; nil dá a cada tenant, quando ele é pedido pela primeira vez,
  # o cofre padrão, cujos membros (members nil) são os do tenant até alguém mexer neles.
  def initialize(vaults, tenants, accounts)
    @vaults = (vaults || []).map { it.stringify_keys.reverse_merge("created_at" => CREATED_AT) }
    @seeded = vaults ? nil : Set.new
    @tenants = tenants
    @accounts = accounts
    @created = 0
  end

  def get(path, account:, role:, tenant_id:, **)
    return colleagues(tenant_id) if path == "/v1/users"

    route = route_of("GET", path, collection: true)
    return mine(account, tenant_id).map { vault_view(it) } unless route[:id]

    members(vault_of(route, account, tenant_id, "GET #{path}"), tenant_id)
  end

  def post(path, body, account:, role:, tenant_id:, **)
    call = "POST #{path}"
    route = route_of("POST", path, collection: true)
    refuse_viewer(role, call)
    return create(body, account, tenant_id, call) unless route[:id]

    add_member(vault_of(route, account, tenant_id, call), body[:user_id], tenant_id, call)
  end

  def patch(path, body, account:, role:, tenant_id:, **)
    call = "PATCH #{path}"
    vault = vault_of(route_of("PATCH", path), account, tenant_id, call)
    refuse_viewer(role, call)
    vault["name"] = valid_name(body[:name], call)
    vault_view(vault)
  end

  def delete(path, account:, role:, tenant_id:, **)
    call = "DELETE #{path}"
    route = route_of("DELETE", path)
    vault = vault_of(route, account, tenant_id, call)
    refuse_viewer(role, call) unless route[:user_id] == account["id"]
    route[:user_id] ? remove_member(vault, route[:user_id], tenant_id, call) : @vaults.delete(vault)
    nil
  end

  private
    # GET e POST agem nas coleções (/v1/vaults e os membros de um cofre); PATCH e DELETE, num item.
    def route_of(verb, path, collection: false)
      route = ROUTE.match(path)
      return route if route && (collection ? !route[:user_id] && route[:id].nil? == route[:members].nil? : route[:id])

      raise ArgumentError, "fake API has no route for #{verb} #{path}, expected /v1/users or /v1/vaults[/{id}[/members[/{user_id}]]]"
    end

    def refuse_viewer(role, call)
      raise MarkuppApi::ReadOnly, "fake API refuses #{call} for role viewer: read_only, expected owner, admin or member" if role == "viewer"
    end

    def tenant_members(tenant_id) = @tenants.tenant(tenant_id)["members"]

    def vaults_in(tenant_id)
      @vaults << { "id" => "vault-1", "tenant_id" => tenant_id, "name" => DEFAULT_NAME, "created_at" => CREATED_AT } if @seeded&.add?(tenant_id)
      @vaults.select { it["tenant_id"] == tenant_id }
    end

    def member_emails(vault) = vault["members"] || tenant_members(vault["tenant_id"]).keys

    def mine(account, tenant_id) = vaults_in(tenant_id).select { member_emails(it).include?(account["email"]) }

    # Como a API: cofre inexistente e cofre de que a conta não é membro são o mesmo vault_forbidden.
    def vault_of(route, account, tenant_id, call)
      mine(account, tenant_id).find { it["id"] == route[:id] } or
        raise MarkuppApi::VaultUnavailable, "fake API refuses #{call}: vault_forbidden, expected one of #{mine(account, tenant_id).map { it["id"] }}"
    end

    def vault_view(vault) = vault.slice("id", "name", "created_at")

    # Como a API: sem espaço nas pontas, de 1 a 100 caracteres. Os ids seguem a ordem: created-vault-1...
    def valid_name(name, call)
      return name if name.is_a?(String) && name == name.strip && name.length.between?(1, NAME_LIMIT)

      raise MarkuppApi::InvalidName, "fake API refuses #{call} with name #{name.inspect}: invalid_name, expected 1 to #{NAME_LIMIT} characters without spaces at the ends"
    end

    def create(body, account, tenant_id, call)
      name = valid_name(body[:name], call)
      @created += 1
      @vaults << { "id" => "created-vault-#{@created}", "tenant_id" => tenant_id, "name" => name, "members" => [ account["email"] ], "created_at" => CREATED_AT }
      vault_view(@vaults.last)
    end

    def members(vault, tenant_id)
      roles = tenant_members(tenant_id)
      member_emails(vault).sort.map { { "id" => @accounts.id_for(it), "email" => it, "role" => roles[it], "joined_at" => CREATED_AT } }
    end

    def email_for(user_id, tenant_id) = tenant_members(tenant_id).keys.find { @accounts.id_for(it) == user_id }

    def add_member(vault, user_id, tenant_id, call)
      email = email_for(user_id, tenant_id) or raise MarkuppApi::NotTenantMember, "fake API refuses #{call}: not_tenant_member, #{user_id.inspect} is not in #{tenant_id}"
      raise MarkuppApi::AlreadyVaultMember, "fake API refuses #{call}: already_vault_member, #{email} is in #{vault["id"]}" if member_emails(vault).include?(email)

      vault["members"] = member_emails(vault) + [ email ]
      members(vault, tenant_id).find { it["email"] == email }
    end

    def remove_member(vault, user_id, tenant_id, call)
      email = member_emails(vault).find { @accounts.id_for(it) == user_id } or
        raise MarkuppApi::VaultMemberNotFound, "fake API refuses #{call}: vault_member_not_found, #{user_id.inspect} is not in #{vault["id"]}"
      vault["members"] = member_emails(vault) - [ email ]
    end

    # Como a API, aberta a qualquer membro do tenant, no mesmo formato dos membros do tenant para todo papel.
    def colleagues(tenant_id)
      tenant_members(tenant_id).sort.map { |email, role| { "id" => @accounts.id_for(email), "email" => email, "role" => role, "joined_at" => CREATED_AT } }
    end
end
