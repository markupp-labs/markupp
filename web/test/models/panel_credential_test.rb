require "test_helper"

class PanelCredentialTest < ActiveSupport::TestCase
  setup do
    @fake_api = FakeMarkuppApi.new
    @session = ActionController::TestSession.new
    @credential = PanelCredential.new(@session, ->(**credentials) { @fake_api.as(**credentials) })
  end

  test "sign_in keeps both tokens and when the access token expires" do
    freeze_time do
      @credential.sign_in({ "access_token" => "access-1", "refresh_token" => "refresh-1", "expires_in" => 900 })

      assert @credential.signed_in?
      assert_equal "refresh-1", @credential.refresh_token
      assert_equal 15.minutes.from_now.to_i, @session[:access_expires_at]
    end
  end

  test "is not signed in on an empty session" do
    assert_not @credential.signed_in?
  end

  test "access_expired? turns true once expires_in has passed" do
    @credential.sign_in({ "access_token" => "access-1", "refresh_token" => "refresh-1", "expires_in" => 900 })

    assert_not @credential.access_expired?
    travel 901.seconds
    assert @credential.access_expired?
  end

  test "account= keeps the id and email for the layout" do
    @credential.account = { "id" => "account-1", "email" => "ana@markupp.dev" }

    assert_equal "ana@markupp.dev", @credential.account_email
  end

  test "sign_out forgets everything in the session" do
    @credential.sign_in({ "access_token" => "access-1", "refresh_token" => "refresh-1", "expires_in" => 900 })
    @session[:tenant_id] = "tenant-1"

    @credential.sign_out

    assert_not @credential.signed_in?
    assert_nil @credential.tenant_id
  end

  test "bearer_api sends the access token and the active tenant" do
    @credential.sign_in(@fake_api.post("/auth/login", email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD))
    @session[:tenant_id] = "tenant-1"

    @credential.bearer_api.get("/notes")

    assert_equal({ access_token: "access-1", tenant_id: "tenant-1", call: "GET /notes" }, @fake_api.received_credentials.sole)
  end

  test "public_api goes without token nor tenant" do
    @session[:tenant_id] = "tenant-1"

    @credential.public_api.post("/auth/logout", refresh_token: "refresh-9")

    assert_equal({ access_token: nil, tenant_id: nil, call: "POST /auth/logout" }, @fake_api.received_credentials.sole)
  end

  test "api renews the access token on its own" do
    assert_instance_of RefreshingMarkuppApi, @credential.api
  end
end
