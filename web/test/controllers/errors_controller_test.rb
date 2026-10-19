require "test_helper"

# Todo erro que o painel trata passa por ele mesmo, no layout público: mascote, mensagem em tom de terminal e link de volta.
class ErrorsControllerTest < ActionDispatch::IntegrationTest
  setup { use_fake_api(notes: []) }

  test "an unknown route answers a styled 404 that points to the login without a session" do
    get "/nowhere/at-all"

    assert_response :not_found
    assert_select "main h1", "404: Nothing at /nowhere/at-all"
    assert_select "main pre[data-mascot=large]"
    assert_select "main a[href=?]", new_session_path, text: "Go to log in"
  end

  test "an unknown route points back to the notes when signed in" do
    sign_in_as

    get "/nowhere"

    assert_response :not_found
    assert_select "main a[href=?]", root_path, text: "Back to your notes"
  end

  test "an unknown route with a file extension is still the styled 404" do
    get "/nowhere.php"

    assert_response :not_found
    assert_select "main h1", "404: Nothing at /nowhere.php"
  end

  test "a request missing its parameters answers a styled 400" do
    sign_in_as

    as_in_production { post notes_path }

    assert_response :bad_request
    assert_select "main h1", "400: Bad request"
  end

  test "a format the page does not speak answers a styled 406" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "" } ])
    sign_in_as

    as_in_production { get note_path("1", format: :xml) }

    assert_response :not_acceptable
    assert_select "main h1", "406: Can't answer in that format"
  end

  test "json errors answer only the status" do
    get "/nowhere", as: :json

    assert_response :not_found
    assert response.body.empty?, "expected an empty json 404, got #{response.body.inspect}"
  end

  test "a form without its authenticity token answers a styled 422" do
    ActionController::Base.allow_forgery_protection = true

    as_in_production { post session_path, params: { session: { email: "ana@example.com", password: "secret" } } }

    assert_response :unprocessable_content
    assert_select "main h1", "422: Change rejected"
  ensure
    ActionController::Base.allow_forgery_protection = false
  end

  test "a bug answers a styled 500 instead of the debug page" do
    use_fake_api.break_with_bug!

    as_in_production { sign_in_as }

    assert_response :internal_server_error
    assert_select "main h1", "500: Something broke on the panel"
    assert_select "main a[href=?]", new_session_path
  end

  private
    # Como em produção: sem a página de debug do Rails, e até o erro sem status conhecido vira página do painel.
    def as_in_production
      config = Rails.application.env_config
      previous = config.slice("action_dispatch.show_exceptions", "action_dispatch.show_detailed_exceptions")
      config.merge!("action_dispatch.show_exceptions" => :all, "action_dispatch.show_detailed_exceptions" => false)
      yield
    ensure
      config.merge!(previous)
    end
end
