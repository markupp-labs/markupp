require "test_helper"

class TokenFormTest < ActiveSupport::TestCase
  INVITE = { id: "invite-1", tenant_id: "tenant-1", email: "caio@markupp.dev", role: "member", token: "tok-1" }.freeze
  ACCEPT_PATH = "/auth/invites/accept"

  setup { @fake_api = FakeMarkuppApi.new(invites: [ INVITE ]) }

  test "submit posts the token and the password and answers the API answer" do
    pair = token_form("tok-1", "senha-nova").submit(@fake_api, ACCEPT_PATH)

    assert_equal %w[access_token refresh_token expires_in], pair.keys
  end

  test "submit with a blank token answers nil and asks for it" do
    form = token_form(" ", "senha-nova")

    assert_nil form.submit(@fake_api, ACCEPT_PATH)
    assert_equal [ "Paste the token from the email." ], form.errors[:token]
  end

  test "submit with a token the API refuses puts the reason on the token" do
    form = token_form("tok-9", "senha-nova")

    assert_nil form.submit(@fake_api, ACCEPT_PATH)
    assert_equal [ "This token is invalid, expired or already used." ], form.errors[:token]
  end

  test "submit with a password the API refuses puts the rule on the password" do
    form = token_form("tok-1", "")

    assert_nil form.submit(@fake_api, ACCEPT_PATH)
    assert_equal [ "Enter a password up to 72 bytes long." ], form.errors[:password]
  end

  test "submit for an email that already has an account puts the reason on base" do
    form = token_form("tok-1", "senha-nova")
    @fake_api = FakeMarkuppApi.new(invites: [ INVITE.merge(email: FakeMarkuppAccounts::DEFAULT_EMAIL) ])

    assert_nil form.submit(@fake_api, ACCEPT_PATH)
    assert form.errors.of_kind?(:base, :account_exists)
    assert_equal [ "This email already has an account. Log in and accept the invite from your tenant list." ], form.errors[:base]
  end

  test "submit says when the server has password login disabled" do
    @fake_api.disable_local_auth!
    form = token_form("tok-1", "senha-nova")

    assert_nil form.submit(@fake_api, ACCEPT_PATH)
    assert_equal [ "Password login is disabled on this server." ], form.errors[:base]
  end

  private
    def token_form(token, password) = TokenForm.new(token:, password:)
end
