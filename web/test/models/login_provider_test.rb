require "test_helper"

class LoginProviderTest < ActiveSupport::TestCase
  test "github and google get their logo" do
    assert_equal "github-logo", LoginProvider.new(id: "github", name: "GitHub").icon
    assert_equal "google-logo", LoginProvider.new(id: "google", name: "Google").icon
  end

  test "another provider has no logo and keeps its name" do
    assert_nil LoginProvider.new(id: "keycloak", name: "Keycloak").icon
  end

  test "a provider creates accounts unless the server says otherwise" do
    assert LoginProvider.new(id: "github", name: "GitHub").registration_open
    fake_api = FakeMarkuppApi.new(login_providers: FakeMarkuppExternalLogin::DEFAULT_PROVIDERS)
    fake_api.close_registration_for!("github")

    assert_equal [ false ], LoginProvider.all(fake_api).map(&:registration_open)
  end
end
