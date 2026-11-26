require "test_helper"

# Configurações, seção Conta: as sessões de login da conta, com a atual marcada, e o encerrar das outras (ADR-0061).
module Settings
  class SessionsControllerTest < ActionDispatch::IntegrationTest
    FIREFOX = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0".freeze
    SESSIONS = [
      { id: "s-atual", current: true, ip: "203.0.113.7", user_agent: FIREFOX },
      { id: "s-celular", ip: "198.51.100.9", user_agent: "Mozilla/5.0 (Linux; Android 15) Chrome/150.0 Mobile Safari/537.36" }
    ].freeze

    test "the account page lists the sessions, this one marked and without end" do
      use_fake_api(sessions: SESSIONS)
      sign_in_as

      get settings_account_path

      assert_select "main h2", "Sessions"
      assert_select "#account_sessions li[data-session=s-atual]", text: /Firefox on Linux.*203\.0\.113\.7.*This session/m
      assert_select "#account_sessions li[data-session=s-atual] form", count: 0
      assert_select "#account_sessions li[data-session=s-celular][data-context-menu-target=item]", text: /Chrome on Android.*198\.51\.100\.9/m do
        assert_select "form[action=?] button.btn-danger[data-context-menu-delete][data-turbo-confirm=?]",
                      settings_session_path("s-celular"), "End the session of Chrome on Android?", text: "End"
      end
    end

    test "ending a session goes back to the account with a notice" do
      api = use_fake_api(sessions: SESSIONS)
      sign_in_as

      delete settings_session_path("s-celular")

      assert_redirected_to settings_account_path
      assert_equal "Session ended. That device signs out within minutes.", flash[:notice]
      assert_equal [ "s-atual" ], api.session_ids
    end

    test "a session that is already gone says so" do
      use_fake_api(sessions: SESSIONS)
      sign_in_as

      delete settings_session_path("s-velha")

      assert_redirected_to settings_account_path
      assert_equal "That session is no longer active.", flash[:alert]
    end
  end
end
