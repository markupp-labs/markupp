require "test_helper"

class SettingsHelperTest < ActionView::TestCase
  attr_accessor :settings_section

  test "role_options pairs the label and value of each role the tenant can assign" do
    tenant = Tenant.new(id: "tenant-1", name: "Acme research", role: "admin")

    assert_equal [ %w[admin admin], %w[member member], %w[viewer viewer] ], role_options(tenant)
  end

  test "chosen_theme reads a known theme from the cookie" do
    cookies[:theme] = "dark"

    assert_equal "dark", chosen_theme
  end

  test "chosen_theme follows the system without a cookie or with an unknown theme" do
    assert_nil chosen_theme

    cookies[:theme] = "sepia"
    assert_nil chosen_theme
  end

  test "settings_nav_link marks only the open section as the current page" do
    self.settings_section = "accounts"

    assert_equal "page", nav_link("accounts", "/settings/account")["aria-current"]
    assert_nil nav_link("appearances", "/settings/appearance")["aria-current"]
    assert_equal [ "Appearance", "/settings/appearance" ], nav_link("appearances", "/settings/appearance").then { [ it.text, it["href"] ] }
  end

  private
    def nav_link(section, path) = Nokogiri::HTML5.fragment(settings_nav_link(section, path)).at("a")
end
