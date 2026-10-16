require "test_helper"

class SettingsHelperTest < ActionView::TestCase
  test "role_options pairs the label and value of each role the tenant can assign" do
    tenant = Tenant.new(id: "tenant-1", name: "Acme research", role: "admin")

    assert_equal [ %w[admin admin], %w[member member], %w[viewer viewer] ], role_options(tenant)
  end
end
