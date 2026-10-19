require "application_system_test_case"

class TenantsTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  test "choosing a tenant card opens its default note, then the sidebar switches it" do
    use_fake_api(tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
                            { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" } } ])
    sign_in_through_form

    assert_selector "h1", text: "Tenants"
    click_on "Home lab"
    assert_selector "nav #current_tenant", text: /Home lab.*member/m
    assert_field "Path", with: "/"

    click_on "Switch tenant"
    within("#tenant_switcher") { click_on "Acme research" }

    assert_selector "nav #current_tenant", text: /Acme research.*owner/m
  end

  test "a new account creates its first tenant and lands on its notes" do
    use_fake_api(tenants: [])
    sign_in_through_form

    assert_text "Create your first tenant"
    fill_in "Name", with: "Acme research"
    click_on "Create"

    assert_text "Created Acme research."
    assert_selector "nav #current_tenant", text: /Acme research.*owner/m
    assert_field "Path", with: "/"
  end
end
