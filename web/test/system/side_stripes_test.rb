require "application_system_test_case"

# Faixa colorida numa lateral só (borda esquerda de destaque) não faz parte do visual do painel: um bloco tem a
# mesma borda nos quatro lados, ou nenhuma.
class SideStripesTest < ApplicationSystemTestCase
  BORDERS_SCRIPT = <<~JS.freeze
    const style = getComputedStyle(arguments[0])
    return [ style.borderLeftWidth, style.borderLeftColor, style.borderTopWidth, style.borderTopColor ]
  JS

  test "an embed has the same border on every side" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "![[ideas]]\n" },
                          { id: "2", path: "ideas.md", content: "Idea body\n" } ])
    sign_in_through_form
    visit note_path("1")
    click_on "Read"

    assert_no_side_stripe find(".note-preview .note-embed", text: "Idea body")
  end

  test "a community group in the explorer has no colored stripe" do
    use_fake_api(notes: [ { id: "hub", path: "hub.md", content: "# Hub\n", degree: 1, pagerank: 0.5, community: 2 },
                          { id: "leaf", path: "leaf.md", content: "# Leaf\n", degree: 1, pagerank: 0.1, community: 2 } ])
    sign_in_through_form
    visit note_path("leaf")
    select "By community", from: "Explorer view"

    assert_equal "0px", border_sides(find(".community-notes", match: :first)).first
  end

  private
    def border_sides(element)
      evaluate_script("(function() { #{BORDERS_SCRIPT} }).apply(null, arguments)", element)
    end

    def assert_no_side_stripe(element)
      left_width, left_color, top_width, top_color = border_sides(element)
      assert_equal [ top_width, top_color ], [ left_width, left_color ], "left border differs from the top one"
    end
end
