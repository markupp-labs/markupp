# Fica no lugar do MarkuppApi nos testes, sem rede: repassa cada chamada à fake do assunto.
class FakeMarkuppApi
  attr_reader :received_credentials

  delegate :put_requests, :post_requests, to: :@notes
  delegate :expire_access_tokens!, :revoke_refresh_tokens!, :refresh_token_valid?, to: :@accounts
  delegate :disable_local_auth!, :hold_refreshes_together!, to: :@auth
  delegate :assign_role!, :deny_notes_in!, to: :@tenants
  delegate :sent_tokens, to: :@account_email

  # email_enabled: false é o servidor sem SMTP, que devolve o token do convite emitido e recusa os fluxos de email da conta.
  def initialize(notes: [], unreachable: false, conflict: false, accounts: FakeMarkuppAccounts::DEFAULT_ACCOUNTS,
                 tenants: FakeMarkuppTenants::DEFAULT_TENANTS, invites: [], email_enabled: true)
    @accounts = FakeMarkuppAccounts.new(accounts)
    @tenants = FakeMarkuppTenants.new(tenants, invites)
    @auth = FakeMarkuppAuth.new(@accounts, @tenants)
    @tenant_admin = FakeMarkuppTenantAdmin.new(@tenants, @accounts, email_enabled:)
    @account_email = FakeMarkuppAccountEmail.new(@accounts, @auth, email_enabled:)
    @notes = FakeMarkuppNotes.new(notes:, conflict:)
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

  def put(path, body) = receive("PUT", path, body)

  def patch(path, body) = receive("PATCH", path, body)

  def post(path, body) = receive("POST", path, body)

  def delete(path) = receive("DELETE", path)

  protected
    attr_writer :credentials

  private
    def receive(verb, path, *body)
      call = "#{verb} #{path}"
      @slow_calls.pass(call)
      raise "fake API bug on #{call}" if @bug
      raise MarkuppApi::Error, "fake API unreachable on #{call}" if @unreachable || (@notes_unreachable && path.start_with?("/notes"))

      @received_credentials << @credentials.merge(call:) if @credentials
      account = authenticate(call)
      membership = tenant_membership(path, account, call) || {}
      fake_for(call, path).public_send(verb.downcase, path, *body, account:, role: membership["role"], tenant_id: membership["id"])
    end

    # Só notas e administração agem num tenant; chamada de confiança, sem conta, não tem papel.
    def tenant_membership(path, account, call)
      return unless account && path.start_with?("/notes", "/users")

      @tenants.membership_for(account, @credentials[:tenant_id], call)
    end

    def authenticate(call)
      return if @credentials.nil? || @auth.public?(call) || @account_email.public?(call)

      @accounts.account_for_access(@credentials[:access_token], call)
    end

    def fake_for(call, path)
      return @notes if path.start_with?("/notes")
      return @account_email if path.start_with?("/auth/password/", "/auth/email/")
      return @auth if path.start_with?("/auth/")
      return @tenants if path.start_with?("/me/") || path == "/tenants"
      return @tenant_admin if path.start_with?("/users")

      raise ArgumentError, "fake API has no route for #{call}, expected /notes, /auth, /me, /tenants or /users"
    end
end
