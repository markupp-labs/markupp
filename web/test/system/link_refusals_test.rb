require "application_system_test_case"

# Recusas de ligação nas configurações do cofre, pelo teclado como as listas de membros: j e k andam pelas linhas e
# Delete na linha com o foco desfaz a recusa dela.
class LinkRefusalsTest < ApplicationSystemTestCase
  NOTES = [ { id: "plan", path: "plan.md", content: "" }, { id: "roadmap", path: "roadmap.md", content: "" },
            { id: "ideas", path: "ideas.md", content: "" } ].freeze
  REFUSALS = [ { id: "r2", note_ids: %w[ideas plan] }, { id: "r1", note_ids: %w[roadmap plan] } ].freeze

  setup do
    use_fake_api(notes: NOTES, link_refusals: REFUSALS)
    sign_in_through_form
  end

  test "j moves to the next refusal and delete undoes it" do
    visit settings_link_refusals_path
    find("#link_refusals > li[tabindex='0']").execute_script("this.focus()")

    send_keys "j"
    assert_selector "#link_refusals > li:focus", text: "roadmap"
    send_keys :delete

    assert_text "Refusal undone."
    assert_no_selector "#link_refusals li", text: "roadmap"
    assert_selector "#link_refusals li", text: "ideas"
  end
end
