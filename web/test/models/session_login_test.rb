require "test_helper"

class SessionLoginTest < ActiveSupport::TestCase
  setup { @fake_api = FakeMarkuppApi.new }

  test "submit answers the token pair for the right email and password" do
    pair = login(FakeMarkuppAccounts::DEFAULT_EMAIL, FakeMarkuppAccounts::DEFAULT_PASSWORD).submit(@fake_api)

    assert_equal %w[access_token refresh_token expires_in], pair.keys
  end

  test "submit with a wrong password answers nil with the error on base" do
    session_login = login(FakeMarkuppAccounts::DEFAULT_EMAIL, "senha-errada")

    assert_nil session_login.submit(@fake_api)
    assert_equal [ "Email or password is wrong." ], session_login.errors[:base]
  end

  test "submit with a blank field reads as wrong credentials" do
    session_login = login("", "")

    assert_nil session_login.submit(@fake_api)
    assert_equal [ "Email or password is wrong." ], session_login.errors[:base]
  end

  test "submit says when the server has password login disabled" do
    @fake_api.disable_local_auth!
    session_login = login(FakeMarkuppAccounts::DEFAULT_EMAIL, FakeMarkuppAccounts::DEFAULT_PASSWORD)

    assert_nil session_login.submit(@fake_api)
    assert_equal [ "Password login is disabled on this server." ], session_login.errors[:base]
  end

  private
    def login(email, password) = SessionLogin.new(email:, password:)
end
