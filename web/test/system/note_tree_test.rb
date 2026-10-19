require "application_system_test_case"

# Árvore de notas da barra lateral desenhada como o comando tree.
class NoteTreeTest < ApplicationSystemTestCase
  NOTES = [ { id: "1", path: "work/2026/goals.md", content: "# Goals\n" }, { id: "2", path: "work/plan.md", content: "# Plan\n" },
            { id: "3", path: "home/list.md", content: "" }, { id: "4", path: "ideas.md", content: "" },
            { id: "5", path: "readme.md", content: "" } ].freeze

  setup do
    use_fake_api(notes: NOTES)
    sign_in_through_form
  end

  test "the tree draws the lines of the tree command, folders first and names without .md" do
    visit note_path("1")

    assert_equal [ "├── home/", "├── work/", "│   ├── 2026/", "│   │   └── goals", "│   └── plan", "├── ideas", "└── readme" ], visible_rows
  end

  # Com 13px de fonte e linhas de 24px, os caracteres │ ├ └ não alcançam a altura da linha e a guia ficava com vãos. As
  # guias são traços de CSS em células de 4 caracteres, da altura toda da linha: a vertical de uma linha encosta na da
  # seguinte (└ para no meio, como no tree).
  test "the vertical lines run unbroken from one row to the next" do
    visit note_path("1")

    rows = all("[role=tree] [role=treeitem]", minimum: 7).select(&:visible?)
    pairs = rows.each_cons(2).map { |above, below| [ edge(above, "bottom"), edge(below, "top") ] }
    assert(pairs.all? { |bottom, top| (bottom - top).abs < 0.5 }, "rows should touch, got #{pairs.inspect}")

    goals = rows.find { it.text.end_with?("goals") }
    heights = evaluate_script(<<~JS, goals)
      [...arguments[0].querySelectorAll(".tree-cell")].map((cell) => [cell.dataset.cell, getComputedStyle(cell, "::before").height])
    JS
    row_height = "#{goals.native.size.height.round}px"
    assert_equal [ [ "pipe", row_height ], [ "pipe", row_height ], [ "elbow", "#{goals.native.size.height.round / 2}px" ] ], heights
  end

  test "the lines are muted text out of the screen reader, with no icons" do
    visit note_path("1")

    assert_no_selector "[role=tree] svg", visible: :all
    guides = all("[role=tree] [role=treeitem] > [aria-hidden=true]:first-child", minimum: 7)
    assert(guides.all? { it.text.match?(/\A[│├└─ ]+\z/) })
    assert_equal css_color("var(--ink-muted)"), guides.first.evaluate_script("getComputedStyle(this).color")
    assert_equal "goals", find("[role=treeitem][aria-current=page]").native.accessible_name
    assert_equal "work", find("[role=treeitem]", text: "work/").native.accessible_name
  end

  test "a folder shows an ascii marker of open or closed and a click toggles it" do
    visit note_path("1")

    home = find("[role=treeitem]", text: "home/")
    assert_equal "false", home["aria-expanded"]
    assert_includes marker(home), "+"
    assert_includes marker(find("[role=treeitem]", text: "work/")), "-"

    home.click
    assert_equal "true", home["aria-expanded"]
    assert_includes marker(home), "-"
    assert_includes visible_rows, "│   └── list"

    home.click
    assert_not_includes visible_rows, "│   └── list"
  end

  test "the rows use 13px text and the current note stays in the ink color" do
    visit note_path("4")

    current = find("[role=treeitem][aria-current=page]", text: "ideas")
    assert_equal "13px", current.evaluate_script("getComputedStyle(this).fontSize")
    assert_equal css_color("var(--ink)"), current.evaluate_script("getComputedStyle(this).color")
  end

  private
    def visible_rows
      evaluate_script(<<~JS)
        [ ...document.querySelectorAll("[role=tree] [role=treeitem]") ].filter((row) => row.checkVisibility()).map((row) => row.textContent.trim())
      JS
    end

    def marker(row)
      row.evaluate_script("getComputedStyle(this.querySelector('.tree-marker'), '::before').content")
    end

    def css_color(value)
      evaluate_script(<<~JS)
        (() => {
          const probe = document.body.appendChild(Object.assign(document.createElement("span"), { style: "color: #{value}" }))
          const color = getComputedStyle(probe).color
          probe.remove()
          return color
        })()
      JS
    end

    def edge(element, side) = evaluate_script("arguments[0].getBoundingClientRect().#{side}", element)
end
