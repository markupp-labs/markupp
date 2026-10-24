require "application_system_test_case"

# Lista de tenants à moda das organizações do Langfuse: um card por tenant, busca que filtra, New tenant num dialog, e
# tudo pelo teclado (setas ou h/j/k/l entre os cards, / na busca, Alt+N no tenant novo, g t de qualquer tela).
class TenantsTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  THREE_TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
                    { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" } },
                    { id: "tenant-3", name: "Field notes", members: { EMAIL => "viewer" } } ].freeze

  # No celular não há teclado: as dicas de tecla somem, como o "Alt N" do New tenant (o / da busca e o Ctrl K da gaveta).
  test "at phone width the key hints are gone" do
    use_fake_api(tenants: THREE_TENANTS.first(2))
    sign_in_through_form
    page.current_window.resize_to(390, 844)

    assert_selector "h1", text: "Tenants"
    assert_no_selector "header kbd", text: "/"
    click_on "Home lab"
    find("[popovertarget=sidebar]").click
    assert_selector "nav#sidebar:popover-open button[data-action='search#open']"
    assert_no_selector "nav#sidebar kbd", text: "Ctrl K"
  ensure
    page.current_window.resize_to(1400, 900)
  end

  test "choosing a tenant card opens its default note, then the sidebar switches it" do
    use_fake_api(tenants: [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
                            { id: "tenant-2", name: "Home lab", members: { EMAIL => "member" } } ])
    sign_in_through_form

    assert_selector "h1", text: "Tenants"
    click_on "Home lab"
    assert_selector "nav #current_tenant", text: /Home lab.*member/m
    assert_field "Path", with: "/"

    click_on "Switch tenant"
    within("#tenant_switcher") { click_on "Acme research" }

    assert_selector "nav #current_tenant", text: /Acme research.*owner/m
  end

  test "a new account creates its first tenant and lands on its notes" do
    use_fake_api(tenants: [])
    sign_in_through_form

    assert_text "Create your first tenant"
    fill_in "Name", with: "Acme research"
    click_on "Create"

    assert_text "Created Acme research."
    assert_selector "nav #current_tenant", text: /Acme research.*owner/m
    assert_field "Path", with: "/"
  end

  test "the keys move between the cards and enter opens the one focused" do
    use_fake_api(tenants: THREE_TENANTS)
    sign_in_through_form
    assert_selector "h1", text: "Tenants"

    send_keys "j"
    assert_card_focused "Acme research"
    send_keys "l"
    assert_card_focused "Home lab"
    send_keys :right
    assert_card_focused "Field notes"
    send_keys "h", :left
    assert_card_focused "Acme research"
    send_keys :end
    assert_card_focused "Field notes"
    send_keys :home, "j"
    assert_card_focused "Home lab"
    send_keys :enter

    assert_selector "nav #current_tenant", text: /Home lab.*member/m
  end

  test "slash goes to the filter, which keeps the matching cards and enters the first on enter" do
    use_fake_api(tenants: THREE_TENANTS)
    sign_in_through_form
    assert_selector "h1", text: "Tenants"

    press_through_cdp("/", "Slash", keycode: 191)
    assert_selector "input[aria-label='Filter tenants']:focus"
    send_keys "LAB"
    assert_equal [ "Home lab" ], all("#tenant_cards .tenant-card-open").map(&:text)
    send_keys "x"
    assert_text "No tenant matches"
    send_keys :backspace, :down
    assert_card_focused "Home lab"
    send_keys :up
    assert_selector "input[aria-label='Filter tenants']:focus"
    send_keys :enter

    assert_selector "nav #current_tenant", text: /Home lab.*member/m
  end

  test "alt+n opens the new tenant dialog, which creates it and opens its notes" do
    use_fake_api(tenants: THREE_TENANTS)
    sign_in_through_form
    assert_selector "h1", text: "Tenants"

    send_keys [ :alt, "n" ]
    assert_selector "dialog#new_tenant[open]"
    within("dialog#new_tenant[open]") do
      assert_selector "input[name='tenant[name]']:focus"
      send_keys "Lab journal", :enter
    end

    assert_text "Created Lab journal."
    assert_selector "nav #current_tenant", text: /Lab journal.*owner/m
  end

  test "a blank name keeps the dialog open with the rule, and esc closes it" do
    use_fake_api(tenants: THREE_TENANTS)
    sign_in_through_form
    assert_selector "button[aria-haspopup=dialog]", text: "New tenant"
    click_on "New tenant"
    within("dialog#new_tenant[open]") do
      find_field("Name").execute_script("this.removeAttribute('required')")
      fill_in "Name", with: "  "
      click_on "Create"
    end

    assert_selector "dialog#new_tenant[open] #tenant_name_error", text: "Use 1 to 100 characters."
    send_keys :escape
    assert_no_selector "dialog#new_tenant[open]"
  end

  test "the gear of a card enters that tenant on its general settings and the envelope on its invites" do
    use_fake_api(tenants: THREE_TENANTS)
    sign_in_through_form
    assert_selector "button[aria-label='Acme research settings']"
    click_on "Acme research settings"

    assert_selector "main h1", text: "General"
    assert_selector "nav #current_tenant", text: /Acme research.*owner/m
    send_keys "g", "t"
    click_on "Invites of Acme research"

    assert_selector "main h1", text: "Invites"
  end

  test "the gear of a viewer card enters that tenant on its general settings too" do
    use_fake_api(tenants: THREE_TENANTS)
    sign_in_through_form
    assert_selector "button[aria-label='Field notes settings']"
    assert_no_selector "button[aria-label='Invites of Field notes']"
    click_on "Field notes settings"

    assert_selector "main h1", text: "General"
    assert_selector "nav #current_tenant", text: /Field notes.*viewer/m
  end

  test "g then t opens the tenants from a note, and so does all tenants in the switcher" do
    use_fake_api(tenants: THREE_TENANTS)
    sign_in_through_form
    assert_selector "h1", text: "Tenants"
    click_on "Acme research"
    assert_field "Path", with: "/"
    execute_script("document.activeElement.blur()")

    send_keys "g", "t"
    assert_selector "h1", text: "Tenants"
    within("main") { click_on "Acme research" }
    click_on "Switch tenant"
    assert_selector "#tenant_switcher a", text: "All tenants"
    within("#tenant_switcher") { click_on "All tenants" }

    assert_selector "h1", text: "Tenants"
  end

  test "esc goes back to the note the tenants were opened from, but not from the filter" do
    use_fake_api(tenants: THREE_TENANTS, notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form
    click_on "Acme research"
    visit note_path("1")
    assert_field "Path", with: "/plan"
    execute_script("document.activeElement.blur()")
    send_keys "g", "t"
    assert_selector "h1", text: "Tenants"

    find("input[aria-label='Filter tenants']").send_keys("lab", :escape)
    assert_selector "h1", text: "Tenants"
    execute_script("document.activeElement.blur()")
    send_keys :escape

    assert_current_path note_path("1")
  end

  # Com tenant ativo a barra é a das notas, mas Alt+N nesta tela continua sendo o New tenant, e a paleta não oferece a
  # nota nova, que cairia no mesmo Alt+N.
  test "with an active tenant the tenants show the notes tree and alt+n still opens the new tenant" do
    use_fake_api(tenants: THREE_TENANTS, notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form
    click_on "Acme research"
    assert_field "Path", with: "/"
    execute_script("document.activeElement.blur()")
    send_keys "g", "t"
    assert_selector "h1", text: "Tenants"

    assert_selector "nav#sidebar [role=treeitem]", text: "plan"
    send_keys [ :control, :shift, "p" ]
    assert_selector "dialog#command_palette[open] input:focus"
    assert_no_selector "#command_palette [role=option]", text: "New note"
    send_keys :escape
    send_keys [ :alt, "n" ]

    assert_selector "dialog#new_tenant[open]"
    assert_current_path tenants_path
  end

  private
    def assert_card_focused(name)
      assert_selector "#tenant_cards .tenant-card-open:focus", text: name
    end
end
