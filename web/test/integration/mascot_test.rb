require "test_helper"

# Mascote em braille: grande nas telas públicas e nas páginas de erro, pequeno na marca da barra lateral.
class MascotTest < ActionDispatch::IntegrationTest
  LARGE = "pre[data-mascot=large][aria-hidden=true]"
  SMALL = "pre[data-mascot=small][aria-hidden=true]"

  test "the public pages show the large mascot beside the form" do
    [ new_session_path, new_invite_acceptance_path, new_password_request_path, new_password_reset_path ].each do |path|
      get path

      assert_select LARGE, { count: 1 }, "expected the large mascot on #{path}"
    end
  end

  test "the sidebar brand is the small mascot as the link home, beside the tenant name of the trail" do
    use_fake_api(notes: [])
    sign_in_as

    get root_path

    assert_select "nav #current_tenant a[href=?]", root_path do
      assert_select SMALL, count: 1
    end
    assert_select "nav #current_tenant button[popovertarget=tenant_switcher]", text: "Acme research"
    assert_select "nav #current_tenant", text: /markupp/, count: 0
    assert_select LARGE, count: 0
  end

  test "without a tenant the sidebar shows the brand with the word markupp" do
    use_fake_api(tenants: [])
    sign_in_as

    get tenants_path

    assert_brand_without_tenant
  end

  test "a note that is not found shows the large mascot" do
    use_fake_api(notes: [])
    sign_in_as

    get note_path("missing")

    assert_response :not_found
    assert_select "main #{LARGE}", count: 1
  end

  test "an unreachable API shows the large mascot" do
    api = use_fake_api
    sign_in_as
    api.cut_connection!

    get root_path

    assert_response :bad_gateway
    assert_select "main #{LARGE}", count: 1
    assert_brand_without_tenant
  end

  test "an unreachable API on the login shows the mascot only once" do
    use_fake_api(unreachable: true)

    sign_in_as

    assert_response :bad_gateway
    assert_select LARGE, count: 1
  end

  private
    def assert_brand_without_tenant
      assert_select "nav a[href=?]", root_path, text: /markupp/ do
        assert_select SMALL, count: 1
      end
    end
end
