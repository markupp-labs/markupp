# Fica no lugar do MarkuppApi nos testes, sem rede: repassa cada chamada à fake do assunto.
class FakeMarkuppApi
  attr_reader :received_credentials

  delegate :put_requests, :post_requests, to: :@notes
  delegate :expire_access_tokens!, :revoke_refresh_tokens!, :refresh_token_valid?, to: :@accounts
  delegate :disable_local_auth!, to: :@auth

  def initialize(notes: [], unreachable: false, conflict: false, accounts: FakeMarkuppAccounts::DEFAULT_ACCOUNTS)
    @accounts = FakeMarkuppAccounts.new(accounts)
    @auth = FakeMarkuppAuth.new(@accounts)
    @notes = FakeMarkuppNotes.new(notes:, conflict:)
    @unreachable = unreachable
    @received_credentials = []
  end

  # Visão da mesma fake com a credencial de um cliente, como MarkuppApi.new(url, access_token:, tenant_id:).
  # Chamada direta, sem as, é de confiança e não confere token: é o que os testes de modelo usam.
  def as(access_token: nil, tenant_id: nil)
    clone.tap { |view| view.credentials = { access_token:, tenant_id: } }
  end

  # Cenário: a API cai depois do login.
  def cut_connection! = @unreachable = true

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
      raise MarkuppApi::Error, "fake API unreachable on #{call}" if @unreachable

      @received_credentials << @credentials.merge(call:) if @credentials
      fake_for(call, path).public_send(verb.downcase, path, *body, account: authenticate(call))
    end

    def authenticate(call)
      return if @credentials.nil? || @auth.public?(call)

      @accounts.account_for_access(@credentials[:access_token], call)
    end

    def fake_for(call, path)
      return @notes if path.start_with?("/notes")
      return @auth if path.start_with?("/auth/")

      raise ArgumentError, "fake API has no route for #{call}, expected /notes or /auth"
    end
end
