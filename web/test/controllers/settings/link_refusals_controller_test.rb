require "test_helper"

# Configurações do cofre ativo, seção Link refusals: as recusas de ligação, a mais nova primeiro, com as duas notas e a
# data, e o desfazer de cada uma. O viewer só lê.
module Settings
  class LinkRefusalsControllerTest < ActionDispatch::IntegrationTest
    NOTES = [ { id: "plan", path: "work/plan.md", content: "" }, { id: "roadmap", path: "roadmap.md", content: "" },
              { id: "ideas", path: "ideas.md", content: "" } ].freeze
    REFUSALS = [ { id: "r2", note_ids: %w[ideas plan], created_at: "2026-11-03T12:00:00Z" },
                 { id: "r1", note_ids: %w[roadmap plan], created_at: "2026-11-01T12:00:00Z" } ].freeze
    VIEWER_TENANTS = [ FakeMarkuppTenants::DEFAULT_TENANTS.first.merge(members: { FakeMarkuppAccounts::DEFAULT_EMAIL => "viewer" }) ].freeze

    def sign_in_with(refusals: REFUSALS, **options)
      use_fake_api(notes: NOTES, link_refusals: refusals, **options)
      sign_in_as
    end

    test "the refusals of the vault show newest first, both notes linking to them, with the date and undo" do
      sign_in_with

      get settings_link_refusals_path

      assert_response :success
      assert_select "main h1", "Link refusals"
      assert_select "main section h1 + p", "Notes"
      assert_select "nav a[aria-current=page][href=?]", settings_link_refusals_path, text: "Link refusals"
      rows = css_select("main ul#link_refusals > li:not([aria-hidden])")
      assert_equal [ %w[ideas plan], %w[plan roadmap] ], rows.map { |row| row.css("a").map(&:text) }
      assert_select rows.first, "a[href=?]", note_path("ideas")
      assert_match(/Nov 03/, rows.first.text)
      assert_select rows.first, "form[action=?] button[data-context-menu-delete][aria-label=?]", settings_link_refusal_path("r2"),
                    "Undo the refusal between ideas and plan", text: "Undo"
    end

    test "a vault without refusals says how to refuse one" do
      sign_in_with(refusals: [])

      get settings_link_refusals_path

      assert_select "main ul#link_refusals", count: 0
      assert_select "main", text: /No refused links/
    end

    test "a viewer reads the refusals without undo" do
      sign_in_with(tenants: VIEWER_TENANTS)

      get settings_link_refusals_path

      assert_select "main ul#link_refusals > li:not([aria-hidden])", 2
      assert_select "main button", text: "Undo", count: 0
    end

    test "undo takes the refusal away and says the notes may link again" do
      sign_in_with

      delete settings_link_refusal_path("r2")

      assert_redirected_to settings_link_refusals_path
      follow_redirect!
      assert_select "[role=status]", "Refusal undone. These notes may get an automatic link again shortly."
      assert_select "main ul#link_refusals a", text: "ideas", count: 0
    end

    test "undo of a refusal already undone says so on top of the list" do
      sign_in_with

      delete settings_link_refusal_path("r9")

      assert_response :not_found
      assert_select "main [role=alert]", "That refusal was already undone."
    end

    test "the vault settings and the palette lead to the refusals" do
      sign_in_with

      get settings_vault_path

      assert_select "nav a[href=?]", settings_link_refusals_path, text: "Link refusals"
      assert_select "#command_palette_options [role=option][data-command-href=?]", settings_link_refusals_path, text: /Vault link refusals/
    end
  end
end
