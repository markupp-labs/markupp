require "test_helper"

class LoginProviderTest < ActiveSupport::TestCase
  test "github and google get their logo" do
    assert_equal "github-logo", LoginProvider.new(id: "github", name: "GitHub").icon
    assert_equal "google-logo", LoginProvider.new(id: "google", name: "Google").icon
  end

  test "another provider has no logo and keeps its name" do
    assert_nil LoginProvider.new(id: "keycloak", name: "Keycloak").icon
  end
end
