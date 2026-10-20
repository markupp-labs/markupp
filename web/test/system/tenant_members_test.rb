require "application_system_test_case"

class TenantMembersTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  setup do
    use_fake_api(tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner", "caio@markupp.dev" => "member" } } ])
    sign_in_through_form
  end

  test "an owner opens the settings from the sidebar footer and changes a role" do
    within("nav > footer") { click_on "Settings" }
    within("main nav") { click_on "Members" }

    assert_selector "h1", text: "Members"
    select "viewer", from: "Role for caio@markupp.dev"
    within("li", text: "caio@markupp.dev") { click_on "Save" }

    assert_text "Role updated."
    assert_select "Role for caio@markupp.dev", selected: "viewer"
  end

  test "an owner removes a member after confirming" do
    visit settings_members_path

    within("li", text: "caio@markupp.dev") { click_on "Remove" }
    assert_selector "dialog#confirm_dialog[open][aria-label=Remove]"
    accept_panel_confirm("Remove caio@markupp.dev from Acme research?", button: "Remove") { nil }

    assert_text "Member removed."
    assert_no_text "caio@markupp.dev"
  end
end
