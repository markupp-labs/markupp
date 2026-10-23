# Fica no lugar do MarkuppApi nos testes, sem rede: repassa cada chamada à fake do assunto.
class FakeMarkuppApi
  OAUTH_REQUESTS = "/v1/oauth/requests/".freeze

  attr_reader :received_credentials

  delegate :put_requests, :post_requests, to: :@notes
  delegate :expire_access_tokens!, :revoke_refresh_tokens!, :refresh_token_valid?, to: :@accounts
  delegate :disable_local_auth!, :hold_refreshes_together!, to: :@auth
  delegate :assign_role!, :deny_notes_in!, :delete_tenant!, to: :@tenants
  delegate :sent_tokens, to: :@account_email
  delegate :received_etags, :graph_queries, :change_graph!, to: :@graph
  delegate :decisions, to: :@oauth_requests, prefix: :oauth

  # email_enabled: false é o servidor sem SMTP, que devolve o token do convite emitido e recusa os fluxos de email da conta.
  # graph é o cenário das ligações, comunidades e grafos sob /v1 (ver FakeMarkuppGraph); oauth_requests, o dos pedidos
  # de autorização OAuth (ver FakeMarkuppOAuthRequests); vaults, o dos cofres de cada tenant (ver FakeMarkuppVaults).
  def initialize(notes: [], unreachable: false, conflict: false, accounts: FakeMarkuppAccounts::DEFAULT_ACCOUNTS,
                 tenants: FakeMarkuppTenants::DEFAULT_TENANTS, invites: [], email_enabled: true, graph: {}, oauth_requests: [], vaults: nil)
    @accounts = FakeMarkuppAccounts.new(accounts)
    @tenants = FakeMarkuppTenants.new(tenants, invites)
    @auth = FakeMarkuppAuth.new(@accounts, @tenants)
    @tenant_admin = FakeMarkuppTenantAdmin.new(@tenants, @accounts, email_enabled:)
    @tenant_deletion = FakeMarkuppTenantDeletion.new(@tenants)
    @account_email = FakeMarkuppAccountEmail.new(@accounts, @auth, email_enabled:)
    @notes = FakeMarkuppNotes.new(notes:, conflict:)
    @note_index = FakeMarkuppNoteIndex.new(@notes)
    @graph = FakeMarkuppGraph.new(graph)
    @oauth_requests = FakeMarkuppOAuthRequests.new(oauth_requests)
    @vaults = FakeMarkuppVaults.new(vaults, @tenants, @accounts)
    @unreachable = unreachable
    @slow_calls = FakeSlowCalls.new(timeout: 10)
    @received_credentials = []
  end

  # Cenário: a API demora nas chamadas que começam com call_prefix ("GET /notes/2") até release_calls!.
  def hold_calls!(call_prefix) = @slow_calls.hold(call_prefix)

  def release_calls! = @slow_calls.release

  # Visão da mesma fake com a credencial de um cliente, como MarkuppApi.new(url, access_token:, tenant_id:).
  # Chamada direta, sem as, é de confiança e não confere token: é o que os testes de modelo usam.
  def as(access_token: nil, tenant_id: nil)
    clone.tap { |view| view.credentials = { access_token:, tenant_id: } }
  end

  # Cenário: a API cai depois do login.
  def cut_connection! = @unreachable = true

  # Cenário: só a listagem de notas cai; login, tenants e conta seguem respondendo.
  def cut_notes! = @notes_unreachable = true

  # Cenário: um bug no meio da chamada, que não é erro da API e o painel não trata.
  def break_with_bug! = @bug = true

  def get(path) = receive("GET", path)

  def conditional_get(path, etag:) = receive("GET", path, method: :conditional_get, etag:)

  def put(path, body) = receive("PUT", path, body)

  def patch(path, body) = receive("PATCH", path, body)

  def post(path, body) = receive("POST", path, body)

  def delete(path) = receive("DELETE", path)

  protected
    attr_writer :credentials

  private
    def receive(verb, path, *body, method: verb.downcase, **options)
      call = "#{verb} #{path}"
      @slow_calls.pass(call)
      raise "fake API bug on #{call}" if @bug
      raise MarkuppApi::Error, "fake API unreachable on #{call}" if @unreachable || (@notes_unreachable && path.start_with?("/notes"))

      @received_credentials << @credentials.merge(call:) if @credentials
      account = authenticate(call)
      membership = tenant_membership(path, account, call) || {}
      fake_for(call, path).public_send(method, path, *body, account:, role: membership["role"], tenant_id: membership["id"], **options)
    end

    # Só notas, grafo e administração agem num tenant; chamada de confiança, sem conta, não tem papel. Os pedidos OAuth
    # são da conta, sem tenant.
    def tenant_membership(path, account, call)
      return unless account && path.start_with?("/notes", "/users", "/v1/") && !path.start_with?(OAUTH_REQUESTS)

      @tenants.membership_for(account, @credentials[:tenant_id], call)
    end

    def authenticate(call)
      return if @credentials.nil? || @auth.public?(call) || @account_email.public?(call)

      @accounts.account_for_access(@credentials[:access_token], call)
    end

    def fake_for(call, path)
      return @notes if path.start_with?("/notes") || (call.start_with?("POST ") && FakeMarkuppNoteIndex::VAULT_NOTES.match?(path))
      return @vaults if FakeMarkuppVaults::ROUTE.match?(path)
      return @oauth_requests if path.start_with?(OAUTH_REQUESTS)
      return @note_index if path.start_with?("/v1/search/") || FakeMarkuppNoteIndex::VAULT_NOTES.match?(path)
      return @graph if path.start_with?("/v1/")
      return @account_email if path.start_with?("/auth/password/", "/auth/email/")
      return @auth if path.start_with?("/auth/")
      return @tenants if path.start_with?("/me/") || path == "/tenants"
      return @tenant_deletion if FakeMarkuppTenantDeletion::TENANT_PATH.match?(path)
      return @tenant_admin if path.start_with?("/users")

      raise ArgumentError, "fake API has no route for #{call}, expected /notes, /v1, /auth, /me, /tenants or /users"
    end
end
