require "application_system_test_case"

# Seção da visão Links com mais de uma página da API: j chega ao Show more no fim, Enter traz a página seguinte no lugar
# dele e o foco desce para a primeira linha nova. A contagem diz 100+ enquanto a seção segue e o número real no fim.
class NoteLinksShowMoreTest < ApplicationSystemTestCase
  PLAN = { id: "plan", title: "plan", path: "plan.md" }.freeze

  setup do
    use_fake_api(notes: [ { id: "plan", path: "plan.md", content: "# plan\n" } ], graph: FakeCrowdedNoteLinks.new(PLAN, count: 130).graph)
    sign_in_through_form
    visit note_path("plan")
    assert_field "Path", with: "/plan"
  end

  test "j reaches show more, enter loads the next page and focuses its first row" do
    send_keys [ :control, :shift, "l" ]
    assert_selector "#links_backlinks summary:focus"
    find("#links_backlinks a", text: "backlink 100").execute_script("this.focus()")

    send_keys "j"
    assert_selector "#links_backlinks .links-more a:focus", text: "Show more"
    send_keys :enter

    assert_selector "#links_backlinks a:focus", text: "backlink 101"
    assert_selector "#links_backlinks a", text: "backlink 130"
    assert_no_selector "#links_backlinks .links-more a", text: "Show more"
    send_keys "k"
    assert_selector "#links_backlinks a:focus", text: "backlink 100"
  end

  test "the count says 100+ while the section goes on and the real number once show more reaches the last page" do
    assert_selector "#links_backlinks summary .links-section-count", text: "100+"

    find("#links_backlinks .links-more a", text: "Show more").click

    assert_selector "#links_backlinks a", text: "backlink 130"
    assert_selector "#links_backlinks summary .links-section-count", exact_text: "130"
  end
end
