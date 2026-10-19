require "application_system_test_case"

class SearchesTest < ApplicationSystemTestCase
  test "ctrl+k searches the notes and arrows plus enter open one" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "Ship the plan" },
                          { id: "2", path: "home/list.md", content: "ship groceries" },
                          { id: "3", path: "inbox.md", content: "later" } ])
    sign_in_through_form

    visit root_path
    find("body").send_keys([ :control, "k" ])
    fill_in "Search notes", with: "ship"

    assert_selector "[role=option]", count: 2
    find_field("Search notes").send_keys(:down, :enter)

    assert_field "Path", with: "/home/list"
    assert_selector "nav a[aria-current=page]", text: "list.md"
  end

  test "the sidebar button opens the search too" do
    use_fake_api(notes: [])
    sign_in_through_form

    visit root_path
    click_on "Search"

    assert_selector "dialog[open] input[type=search]:focus"
  end
end
