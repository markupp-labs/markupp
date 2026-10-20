require "application_system_test_case"

# Botão direito no espaço vazio da barra lateral é a raiz: o menu só oferece a nota nova, criada em /. Sem isso o
# navegador abria o menu nativo e não havia como criar na raiz pelo mouse.
class NoteTreeRootMenuTest < ApplicationSystemTestCase
  test "a right click below the tree opens the root menu and the new note starts at the root" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan\n" } ])
    sign_in_through_form
    visit note_path("1")

    right_click_below_the_tree

    assert_equal [ "New note here" ], menu_items
    within("[role=menu]") { click_on "New note here" }
    assert_field "Path", with: "/"
  end

  test "a sidebar with no notes yet opens the root menu too" do
    use_fake_api(notes: [])
    sign_in_through_form
    visit root_path

    right_click_below_the_tree

    assert_equal [ "New note here" ], menu_items
  end

  private
    # Perto do fundo da área que rola na barra lateral, bem abaixo da última linha. O deslocamento do Selenium conta do
    # centro do elemento.
    def right_click_below_the_tree
      area = find("nav#sidebar .sidebar-scroll")
      area.right_click(x: 0, y: (area.native.size.height / 2) - 40)
    end

    def menu_items
      assert_selector "[role=menu]"
      all("[role=menu] [role=menuitem]").map { it.native.accessible_name }
    end
end
