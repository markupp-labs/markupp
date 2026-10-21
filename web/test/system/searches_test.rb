require "application_system_test_case"

# Busca ranqueada no Ctrl+K: o mais relevante primeiro, com o trecho e o termo destacado; no campo, setas ou Ctrl+J e
# Ctrl+K andam (como na paleta); na lista, setas ou j e k; Enter abre.
class SearchesTest < ApplicationSystemTestCase
  setup do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "Ship the plan" },
                          { id: "2", path: "home/list.md", content: "ship groceries, ship soon" },
                          { id: "3", path: "inbox.md", content: "later" } ])
    sign_in_through_form
    visit root_path
  end

  test "ctrl+k searches by rank with the matched term marked, and ctrl+j plus enter open the next hit" do
    open_search("ship")

    assert_equal %w[list plan], all("[role=option] .search-hit-title").map(&:text)
    assert_selector "[role=option][aria-selected=true] .search-hit-snippet mark", text: "ship", count: 2
    selected = find("[role=option][aria-selected=true]")
    assert_equal css_color("var(--accent-soft)"), evaluate_script("getComputedStyle(arguments[0]).backgroundColor", selected)
    find_field("Search notes").send_keys([ :control, "j" ])
    assert_selector "[role=option][aria-selected=true]", text: "plan"
    find_field("Search notes").send_keys([ :control, "k" ], [ :control, "j" ], :enter)

    assert_field "Path", with: "/work/plan"
  end

  test "arrows from the field and j and k in the list move the selection, enter opens it" do
    open_search("ship")
    find_field("Search notes").send_keys(:down, :up)
    assert_selector "[role=option][aria-selected=true]", text: "list"

    find_field("Search notes").send_keys(:tab)
    assert_selector "[role=option][aria-selected=true]:focus", text: "list"
    send_keys "j"
    assert_selector "[role=option][aria-selected=true]:focus", text: "plan"
    send_keys "k", "j", :enter

    assert_field "Path", with: "/work/plan"
  end

  test "a query without matches says so with a hint" do
    open_search("nothing")

    assert_text 'No matches for "nothing"'
    assert_text "Try fewer words"
  end

  test "the empty state stands off the edge of the dialog, its tildes in line with the text of the field" do
    open_search("nothing")
    assert_text 'No matches for "nothing"'

    field = find_field("Search notes")
    text_start = edge(field, "left") + evaluate_script("parseFloat(getComputedStyle(arguments[0]).paddingLeft)", field)
    assert_in_delta text_start, edge(find("#search_results .empty-state pre"), "left"), 0.5
  end

  test "the sidebar button opens the search too" do
    click_on "Search"

    assert_selector "dialog[open] input[type=search]:focus"
  end

  private
    def open_search(query)
      find("body").send_keys([ :control, "k" ])
      fill_in "Search notes", with: query
    end
end
