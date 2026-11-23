require "test_helper"

# A tentativa de login externo na sessão: guarda o que o servidor gerou, vale uma vez e só com o state certo.
class ExternalLoginAttemptTest < ActiveSupport::TestCase
  # Faz o papel da API pública só na rota da tentativa, com a URL que o teste escolhe.
  class FakeAttemptsApi
    def initialize(url) = @url = url

    def post(path, _body)
      raise ArgumentError, "fake API has no route for POST #{path}" unless path == "/v1/login-providers/github/attempts"

      { "authorization_url" => @url, "state" => "s", "code_verifier" => "v", "nonce" => "n" }
    end
  end

  def start(url, session = {})
    ExternalLoginAttempt.start(session, FakeAttemptsApi.new(url), provider: "github", purpose: "login")
  end

  test "start keeps state, verifier and nonce in the session and returns the provider url" do
    session = {}

    url = start("https://github.com/login/oauth/authorize?state=s", session)

    assert_equal "https://github.com/login/oauth/authorize?state=s", url
    assert_equal({ "provider" => "github", "purpose" => "login", "state" => "s", "code_verifier" => "v", "nonce" => "n" },
                 session[ExternalLoginAttempt::SESSION_KEY])
  end

  test "start refuses an authorization url that is not absolute http(s)" do
    [ "javascript:alert(1)", "/auth/github/callback", "//outro.example/x", "nao e url" ].each do |url|
      assert_raises(MarkuppApi::Error, url) { start(url) }
    end
  end

  test "take returns the attempt with the body for the server, once" do
    session = {}
    start("https://github.com/authorize", session)

    attempt = ExternalLoginAttempt.take(session, provider: "github", state: "s")

    assert_equal "login", attempt.purpose
    assert_equal({ provider: "github", code: "c", code_verifier: "v", nonce: "n" }, attempt.callback_body("c"))
    assert_nil ExternalLoginAttempt.take(session, provider: "github", state: "s")
  end

  test "take refuses another state or provider and drops the attempt" do
    [ { provider: "github", state: "outro" }, { provider: "google", state: "s" } ].each do |callback|
      session = {}
      start("https://github.com/authorize", session)

      assert_nil ExternalLoginAttempt.take(session, **callback)
      assert_nil session[ExternalLoginAttempt::SESSION_KEY]
    end
  end
end
