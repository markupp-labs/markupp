require "test_helper"

# Lista de tenants com um tenant ativo, como as configurações: a árvore das notas na barra lateral e um voltar (botão e
# Esc) para a página de onde ela foi aberta.
class TenantsAreaTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  TWO_TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
                  { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" } } ].freeze

  setup do
    use_fake_api(tenants: TWO_TENANTS, notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_as
  end

  test "with an active tenant the sidebar draws the notes tree" do
    patch current_tenant_path, params: { tenant_id: "tenant-1" }

    get tenants_path

    assert_select "nav#sidebar #current_tenant", text: /Acme research/
    assert_select "nav#sidebar [role=tree] [role=treeitem]", text: /plan/
    assert_select "button[popovertarget=sidebar]", text: "Notes"
  end

  test "back leads to the page the tenants were opened from" do
    patch current_tenant_path, params: { tenant_id: "tenant-1" }

    get tenants_path, headers: { "HTTP_REFERER" => "http://www.example.com/notes/1?mode=read" }

    assert_select "main a[href=?][aria-label=?][aria-keyshortcuts=Escape]", "/notes/1?mode=read", "Back"
    assert_select "#shortcut_help dd", "Leave the settings or the tenants"
  end

  test "back ignores a referer from another site and falls back to the root" do
    patch current_tenant_path, params: { tenant_id: "tenant-1" }

    get tenants_path, headers: { "HTTP_REFERER" => "https://evil.example.net/phish" }

    assert_select "main a[href=?][aria-label=?]", root_path, "Back"
  end

  # Tenants, a engrenagem de um card (Members), Esc de volta aos tenants e Esc de novo: a nota, não Members outra vez.
  test "coming back from the settings opened from the tenants keeps where the tenants were opened from" do
    patch current_tenant_path, params: { tenant_id: "tenant-1" }
    get tenants_path, headers: { "HTTP_REFERER" => "http://www.example.com/notes/1" }

    get settings_members_path, headers: { "HTTP_REFERER" => "http://www.example.com/tenants" }
    assert_select "main a[href=?][aria-label=?]", "/tenants", "Back"
    get tenants_path, headers: { "HTTP_REFERER" => "http://www.example.com/settings/members" }

    assert_select "main a[href=?][aria-label=?]", "/notes/1", "Back"
  end

  test "without an active tenant there is no back, the root would come back here" do
    get tenants_path, headers: { "HTTP_REFERER" => "http://www.example.com/session/new" }

    assert_select "main a[aria-label=?]", "Back", count: 0
    assert_select "nav#sidebar [role=tree]", count: 0
  end
end
