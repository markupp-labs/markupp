require "test_helper"

# Explorer da barra lateral: o seletor troca a árvore de pastas por listas planas do cofre (as mais centrais, as mais
# ligadas, as mudadas por último, as órfãs, por comunidade), e a escolha fica no cookie explorer_mode.
class ExplorersControllerTest < ActionDispatch::IntegrationTest
  NOTES = [
    { id: "hub", path: "work/hub.md", content: "", degree: 4, pagerank: 0.5, community: 2, updated_at: 2.hours.ago.iso8601 },
    { id: "leaf", path: "leaf.md", content: "", degree: 1, pagerank: 0.1, community: 2, updated_at: 3.days.ago.iso8601 },
    { id: "pair", path: "pair.md", content: "", degree: 2, pagerank: 0.3, community: 9, updated_at: 5.minutes.ago.iso8601 },
    { id: "lone", path: "lone.md", content: "", degree: 0, pagerank: 0.05, updated_at: 40.days.ago.iso8601 }
  ].freeze
  MODES = [ [ "folders", "Folders" ], [ "central", "Most central" ], [ "linked", "Most linked" ],
            [ "recent", "Recently updated" ], [ "orphans", "Orphans" ], [ "communities", "By community" ] ].freeze

  setup do
    @api = use_fake_api(notes: NOTES)
    sign_in_as
  end

  test "the sidebar offers the explorer views in a select with its shortcut, folders first and chosen by default" do
    get root_path

    assert_select "form[action=?][method=get][data-turbo-frame=explorer_list]", explorer_path do
      assert_select "select#explorer_mode[name=mode][aria-keyshortcuts=Alt+O][title=?]", "Explorer view (Alt+O)"
      assert_equal MODES, css_select("select#explorer_mode option").map { [ it["value"], it.text ] }
      assert_select "option[selected]", text: "Folders"
    end
    assert_select "turbo-frame#explorer_list [role=tree] .tree-guide", minimum: 1
  end

  test "a chosen view is remembered in a cookie and drawn by the next page, flat and with the open note marked" do
    get explorer_path(mode: "central", current: "pair")

    assert_equal "central", cookies[:explorer_mode]
    assert_equal %w[hub pair leaf lone], frame_row_titles
    assert_select "turbo-frame#explorer_list .tree-guide", count: 0
    assert_select "[role=treeitem][aria-current=page]", text: /pair/

    get note_path("leaf")

    assert_select "select#explorer_mode option[selected]", text: "Most central"
    assert_equal %w[hub pair leaf lone], frame_row_titles
    assert_select "[role=treeitem][aria-current=page]", text: /leaf/
  end

  test "the rows keep the path of the note for the menu and show its folder" do
    get explorer_path(mode: "central")

    assert_select "a[role=treeitem][href=?][data-path=?][title=?]", note_path("hub"), "/work/hub", "work/hub.md" do
      assert_select ".explorer-row-folder", text: "work"
    end
  end

  test "most linked shows the degree and recently updated the age of each note" do
    get explorer_path(mode: "linked")

    assert_equal %w[hub pair leaf lone], frame_row_titles
    assert_equal %w[4 2 1 0], css_select(".explorer-row-metric").map { it.text.strip }

    get explorer_path(mode: "recent")

    assert_equal %w[pair hub leaf lone], frame_row_titles
    assert_equal %w[5m 2h 3d 1mo], css_select(".explorer-row-metric").map { it.text.strip }
    assert_select ".explorer-row-metric time[datetime]", count: 4
  end

  test "by community groups the notes under each community, with its color, and the notes without one last" do
    get explorer_path(mode: "communities")

    groups = css_select("[role=tree] > li > [role=group]")
    assert_equal [ "Community around hub, 2 notes", "Community around pair, 1 note", "No community, 1 note" ], groups.map { it["aria-label"] }
    assert_equal [ %w[hub leaf], %w[pair], %w[lone] ], groups.map { |group| group.css("[role=treeitem] .explorer-row-title").map(&:text) }
    assert_equal [ "--community: var(--community-2)", "--community: var(--community-1)", nil ], css_select("[role=tree] > li").map { it["style"] }
  end

  test "orphans says when every note has a link" do
    @api = use_fake_api(notes: NOTES.reject { it[:degree].zero? })
    sign_in_as

    get explorer_path(mode: "orphans")

    assert_select "turbo-frame#explorer_list [role=treeitem]", count: 0
    assert_select "turbo-frame#explorer_list", text: /No orphan notes/
  end

  test "an unknown view falls back to the folders" do
    get explorer_path(mode: "search")

    assert_equal "folders", cookies[:explorer_mode]
    assert_select "turbo-frame#explorer_list [role=tree] .tree-guide", minimum: 1
  end

  test "an unreachable server shows the failure inside the frame" do
    @api.cut_connection!

    get explorer_path(mode: "central"), headers: { "Turbo-Frame" => "explorer_list" }

    assert_response :bad_gateway
    assert_select "turbo-frame#explorer_list", text: /Can't reach the markupp server/
  end

  private
    def frame_row_titles = css_select("turbo-frame#explorer_list [role=treeitem] .explorer-row-title").map(&:text)
end
