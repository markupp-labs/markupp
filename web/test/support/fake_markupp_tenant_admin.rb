# Rotas /users da FakeMarkuppApi: membros e convites do tenant do cabeçalho, com as regras de papel da API
# (só owner e admin entram, ninguém mexe acima do próprio posto, o tenant nunca fica sem owner).
class FakeMarkuppTenantAdmin
  RANKS = %w[viewer member admin owner].freeze
  JOINED_AT = "2026-09-01T12:00:00Z"
  INVITE_PATH = %r{\A/users/invites/(?<id>[^/]+)\z}
  MEMBER_PATH = %r{\A/users/(?<id>[^/]+)\z}

  def initialize(tenants, accounts, email_enabled:)
    @tenants = tenants
    @accounts = accounts
    @email_enabled = email_enabled
    @issued = 0
  end

  def get(path, role:, tenant_id:, **)
    refuse_non_admin(role, "GET #{path}")
    return members(tenant_id) if path == "/users"
    return invites(tenant_id) if path == "/users/invites"

    raise ArgumentError, "fake API has no route for GET #{path}, expected /users or /users/invites"
  end

  def patch(path, body, role:, tenant_id:, **)
    call = "PATCH #{path}"
    refuse_non_admin(role, call)
    raise MarkuppApi::InvalidRequest, "fake API refuses #{call} with role #{body[:role].inspect}, expected one of #{RANKS}" unless RANKS.include?(body[:role])

    change_membership(tenant_id, member_id(path, call), role, body[:role], call)
  end

  def delete(path, role:, tenant_id:, **)
    call = "DELETE #{path}"
    refuse_non_admin(role, call)
    invite = INVITE_PATH.match(path)
    return revoke(tenant_id, invite[:id], call) if invite

    change_membership(tenant_id, member_id(path, call), role, nil, call)
  end

  def post(path, body, role:, tenant_id:, **)
    call = "POST #{path}"
    refuse_non_admin(role, call)
    raise ArgumentError, "fake API has no route for #{call}, expected /users/invites" unless path == "/users/invites"

    issue(tenant_id, body[:email].to_s.strip.downcase, body[:role], role, call)
  end

  private
    def rank(role) = RANKS.index(role).to_i

    def refuse_non_admin(role, call)
      raise MarkuppApi::Forbidden, "fake API refuses #{call} for role #{role.inspect}: forbidden, expected owner or admin" unless %w[owner admin].include?(role)
    end

    def member_id(path, call)
      MEMBER_PATH.match(path)&.[](:id) or raise ArgumentError, "fake API has no route for #{call}, expected /users/{id}"
    end

    def members(tenant_id)
      @tenants.tenant(tenant_id)["members"].sort.map do |email, role|
        { "id" => @accounts.id_for(email), "email" => email, "role" => role, "joined_at" => JOINED_AT }
      end
    end

    # Como CheckMembershipChange: posto do alvo, posto concedido e último owner; new_role nil remove.
    def change_membership(tenant_id, id, actor_role, new_role, call)
      members = @tenants.tenant(tenant_id)["members"]
      email = members.keys.find { @accounts.id_for(it) == id } or
        raise MarkuppApi::MemberNotFound, "fake API has no member #{id.inspect} in #{tenant_id}: member_not_found"
      refuse_above_own(actor_role, [ members[email], new_role ].compact, call)
      refuse_last_owner(members, email, new_role, call)
      new_role ? members[email] = new_role : members.delete(email)
      nil
    end

    def refuse_above_own(actor_role, roles, call)
      above = roles.find { rank(it) > rank(actor_role) }
      raise MarkuppApi::RoleAboveOwn, "fake API refuses #{call}: role_above_own, #{above} is above #{actor_role}" if above
    end

    def refuse_last_owner(members, email, new_role, call)
      return unless members[email] == "owner" && new_role != "owner" && members.values.count("owner") <= 1

      raise MarkuppApi::LastOwner, "fake API refuses #{call}: last_owner, #{email} is the only owner"
    end

    def invites(tenant_id)
      @tenants.invites.select { it["tenant_id"] == tenant_id }.map { invite_view(it) }
    end

    def invite_view(invite) = invite.slice("id", "email", "role", "expires_at", "accepted_at", "created_at").compact

    # Como a API: email com @, papel conhecido e até o do emissor, email que ainda não é membro. Sem SMTP, o token vem junto.
    def issue(tenant_id, email, role, actor_role, call)
      raise MarkuppApi::InvalidRequest, "fake API refuses #{call} with email #{email.inspect}: invalid_request, expected user@domain" unless email.include?("@")
      raise MarkuppApi::InvalidRequest, "fake API refuses #{call} with role #{role.inspect}: invalid_request, expected one of #{RANKS}" unless RANKS.include?(role)

      refuse_above_own(actor_role, [ role ], call)
      raise MarkuppApi::AlreadyMember, "fake API refuses #{call}: already_member, #{email} is in #{tenant_id}" if @tenants.tenant(tenant_id)["members"].key?(email)

      invite = new_invite(tenant_id, email, role)
      @tenants.invites << invite
      @email_enabled ? invite_view(invite) : invite_view(invite).merge("token" => invite["token"])
    end

    # Ids e tokens seguem a ordem de emissão: issued-1, issued-token-1...
    def new_invite(tenant_id, email, role)
      @issued += 1
      { "id" => "issued-#{@issued}", "tenant_id" => tenant_id, "email" => email, "role" => role, "token" => "issued-token-#{@issued}",
        "expires_at" => 7.days.from_now.utc.iso8601, "created_at" => Time.current.utc.iso8601 }
    end

    # Como a API: só convite sem resposta sai; aceito ou de outro tenant é invite_not_found.
    def revoke(tenant_id, id, call)
      invite = @tenants.invites.find { it["id"] == id && it["tenant_id"] == tenant_id && it["accepted_at"].nil? } or
        raise MarkuppApi::InviteNotFound, "fake API refuses #{call}: invite_not_found, no open invite #{id.inspect} in #{tenant_id}"
      @tenants.invites.delete(invite)
      nil
    end
end
