# Contas, bloqueios de IP, histórico e prazo de guarda da FakeMarkuppApi sob /v1/instance, como a API (ADR-0061): só a conta
# com instance_admin entra, o administrador não bloqueia nem apaga a própria conta, e a faixa com o IP de quem pede é
# recusada. scenario: { accounts: [{ id:, email:, blocked:, sessions: [], authorizations: [], sole_owner_tenants: [] }],
# ip_blocks: [{ id:, cidr:, note: }], audit_events: [{ id:, action:, ... }], retention: { days:, host_locked: } }.
class FakeMarkuppInstanceAccess
  include FakeInstanceAdminGuard

  ROUTE = %r{\A/v1/instance/(?:accounts|ip-blocks|audit-events|retention)}
  ACCOUNT = %r{\A/v1/instance/accounts/(?<id>[^/?]+)(?:/(?<kind>sessions|authorizations)(?:/(?<item>[^/]+))?)?\z}
  ADMIN_IP = "127.0.0.1".freeze

  attr_reader :queries

  def self.handles?(path) = ROUTE.match?(path)

  def initialize(scenario)
    @accounts = (scenario[:accounts] || []).map { default_account(it[:id]).merge(it.deep_stringify_keys) }
    @ip_blocks = (scenario[:ip_blocks] || []).map { { "note" => "", "created_at" => "2026-11-20T12:00:00Z" }.merge(it.stringify_keys) }
    @audit_events = (scenario[:audit_events] || []).map { default_event(it[:id]).merge(it.stringify_keys) }
    @retention = { "days" => 365, "host_locked" => false }.merge((scenario[:retention] || {}).stringify_keys)
    @queries = []
  end

  def get(path, account:, **)
    refuse_non_admin(account, "GET #{path}")
    uri = URI.parse(path)
    @queries << uri.query.to_s
    return @retention if uri.path == "/v1/instance/retention"
    return @ip_blocks if uri.path == "/v1/instance/ip-blocks"
    return page(@audit_events, uri) if uri.path == "/v1/instance/audit-events"
    return page(filtered_accounts(uri), uri).map { it.except("sessions", "authorizations", "sole_owner_tenants") } if uri.path == "/v1/instance/accounts"

    account_part(uri.path)
  end

  def patch(path, body, account:, **)
    refuse_non_admin(account, "PATCH #{path}")
    target = find_account(path, account)
    target["blocked"] = body[:blocked]
    target
  end

  def put(path, body, account:, **)
    refuse_non_admin(account, "PUT #{path}")
    raise MarkuppApi::HostLocked, "fake API refuses PUT #{path}: host_locked" if @retention["host_locked"]
    raise MarkuppApi::InvalidRetention, "fake API refuses #{body[:days]} days: invalid_retention" unless (1..3650).cover?(body[:days].to_i)

    @retention.merge!("days" => body[:days].to_i)
  end

  def post(path, body, account:, **)
    refuse_non_admin(account, "POST #{path}")
    cidr = body[:cidr].to_s
    raise MarkuppApi::InvalidIpBlock, "fake API refuses #{cidr.inspect}: invalid_ip_block" unless cidr.match?(%r{\A[0-9a-f.:]+(/\d+)?\z})
    raise MarkuppApi::BlocksOwnIp, "fake API refuses #{cidr}: blocks_own_ip" if cidr.start_with?("0.0.0.0", ADMIN_IP)

    block = { "id" => "b#{@ip_blocks.size + 1}", "cidr" => cidr, "note" => body[:note].to_s, "created_at" => "2026-11-25T12:00:00Z" }
    @ip_blocks << block
    block
  end

  def delete(path, account:, **)
    refuse_non_admin(account, "DELETE #{path}")
    return remove_ip_block(path) if path.start_with?("/v1/instance/ip-blocks/")

    match = ACCOUNT.match(path)
    target = find_account(path, account)
    return remove_item(target, match) if match[:kind]

    @accounts.delete(target)
    nil
  end

  def account(id) = @accounts.find { it["id"] == id }

  def ip_blocks = @ip_blocks

  private
    def default_account(id)
      { "id" => id, "email" => "#{id}@markupp.dev", "email_verified" => true, "login_methods" => [ "local" ],
        "created_at" => "2026-11-01T12:00:00Z", "last_access_at" => nil, "blocked" => false, "sessions" => [],
        "authorizations" => [], "sole_owner_tenants" => [] }
    end

    def default_event(id)
      { "id" => id, "actor_id" => "", "actor_email" => "", "action" => "auth.login", "target" => "", "result" => "success",
        "ip" => "", "user_agent" => "", "occurred_at" => "2026-11-25T12:00:00Z" }
    end

    def filtered_accounts(uri)
      params = Rack::Utils.parse_query(uri.query)
      @accounts.select do |a|
        a["email"].include?(params["q"].to_s) && (params["status"].blank? || a["blocked"] == (params["status"] == "blocked"))
      end
    end

    def page(list, uri)
      params = Rack::Utils.parse_query(uri.query)
      offset = params["offset"].to_i
      list.drop(offset).first((params["limit"] || 50).to_i)
    end

    def account_part(path)
      match = ACCOUNT.match(path) or raise ArgumentError, "fake API has no route for GET #{path}"
      target = find_account(path, nil)
      match[:kind] ? target[match[:kind]] : target.except("sessions", "authorizations")
    end

    def find_account(path, actor)
      id = ACCOUNT.match(path)[:id]
      raise MarkuppApi::CannotTargetSelf, "fake API refuses #{path}: cannot_target_self" if actor && actor["id"] == id && !ACCOUNT.match(path)[:kind]

      account(id) or raise MarkuppApi::AccountNotFound, "fake API has no account #{id.inspect}: account_not_found"
    end

    def remove_item(target, match)
      list = target[match[:kind]]
      gone = list.reject! { it["id"] == match[:item] }
      error = match[:kind] == "sessions" ? MarkuppApi::SessionNotFound : MarkuppApi::AuthorizationNotFound
      raise error, "fake API has no #{match[:kind]} #{match[:item].inspect}" unless gone

      nil
    end

    def remove_ip_block(path)
      id = path.split("/").last
      @ip_blocks.reject! { it["id"] == id } or raise MarkuppApi::IpBlockNotFound, "fake API has no ip block #{id.inspect}"
      nil
    end
end
