require "test_helper"

# Configurações da instância, seção Histórico (ADR-0061): os acessos e as mudanças sensíveis, do mais novo para trás, com
# filtros e páginas.
module Settings
  class InstanceHistoryControllerTest < ActionDispatch::IntegrationTest
    ADMIN_ACCOUNTS = [ { email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, instance_admin: true } ].freeze
    EVENTS = [
      { id: "e1", actor_id: "u-bia", actor_email: "bia@exemplo.org", action: "auth.login", result: "failure",
        ip: "198.51.100.9", user_agent: "curl/8", occurred_at: "2026-11-25T12:00:00Z" },
      { id: "e2", actor_id: "", actor_email: "", action: "auth.login", target: "ninguem@exemplo.org", result: "failure", ip: "198.51.100.9" },
      { id: "e3", actor_id: "u-bia", actor_email: "bia@exemplo.org", action: "auth.password_reset", result: "success" }
    ].freeze

    def sign_in_admin(**scenario)
      fake = use_fake_api(accounts: ADMIN_ACCOUNTS, instance_access: scenario)
      sign_in_as
      fake
    end

    test "the history lists the events with who, what, result and where" do
      sign_in_admin(audit_events: EVENTS)

      get settings_instance_history_path

      assert_select "main h1", "History"
      assert_select "main nav a[href=?][aria-current=page]", settings_instance_history_path
      assert_select "#audit_events li[data-audit-event=e1]", text: /bia@exemplo\.org.*Sign in.*Failed.*198\.51\.100\.9.*curl/m
      assert_select "#audit_events li[data-audit-event=e2]", text: /ninguem@exemplo\.org.*Sign in.*Failed/m
      assert_select "#audit_events li[data-audit-event=e3]", text: /Password reset/
    end

    test "the filters go to the API" do
      fake = sign_in_admin(audit_events: EVENTS)

      get settings_instance_history_path(account: "u-bia", result: "failure", ip: "198.51.100.9", since: "2026-11-01")

      assert_includes fake.instance_access.queries, "account=u-bia&result=failure&ip=198.51.100.9&since=2026-11-01T00%3A00%3A00Z&limit=50&offset=0"
      assert_select "form[role=search] select[name=result] option[selected][value=failure]"
    end

    test "an unknown action shows as it came" do
      sign_in_admin(audit_events: [ { id: "e9", action: "plugin.custom" } ])

      get settings_instance_history_path

      assert_select "#audit_events li[data-audit-event=e9]", text: /plugin\.custom/
    end

    test "with no events it says how long they are kept" do
      sign_in_admin(retention: { days: 90 })

      get settings_instance_history_path

      assert_select "main", text: /No events\. The history keeps the last 90 days\./
    end
  end
end
