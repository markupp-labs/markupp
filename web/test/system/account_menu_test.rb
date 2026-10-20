require "application_system_test_case"

# Rodapé da barra lateral numa linha: o email abre o menu da conta (quem está logado e o sair), à direita só os ícones
# de tema e configurações, que é a única entrada das configurações. O menu anda pelo teclado como o da árvore.
class AccountMenuTest < ApplicationSystemTestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL

  setup do
    use_fake_api(notes: [])
    sign_in_through_form
  end

  test "the footer fits in a single line" do
    tops = evaluate_script(<<~JS)
      [...document.querySelectorAll("nav > footer :is(button[popovertarget=account_menu], form[class^=theme-to] button, a[aria-label=Settings])")]
        .filter((item) => item.checkVisibility())
        .map((item) => Math.round(item.getBoundingClientRect().top + item.offsetHeight / 2))
    JS

    assert_equal 3, tops.size, "expected the email, one theme icon and the gear, got #{tops.size} items"
    assert_equal 1, tops.uniq.size, "expected one line, got middles at #{tops.inspect}"
  end

  # Aberto pelo mouse, nenhum item ganha o foco: um Enter logo depois não sai da conta. As setas entram no menu.
  test "a click on the email opens the menu without focusing log out and esc gives the focus back" do
    click_on EMAIL

    assert_selector "#account_menu:popover-open [role=menu]"
    assert_no_selector "[role=menuitem]:focus"
    send_keys :enter
    assert_selector "#account_menu:popover-open"
    send_keys :down
    assert_selector "[role=menuitem]:focus", text: "Log out"
    send_keys :escape

    assert_no_selector "#account_menu:popover-open"
    assert_selector "button[popovertarget=account_menu]:focus"
  end

  test "clicking the email again closes the menu" do
    click_on EMAIL
    assert_selector "#account_menu:popover-open"
    click_on EMAIL

    assert_no_selector "#account_menu:popover-open"
  end

  test "the menu shows the whole email and only log out, the gear is the way to the settings" do
    click_on EMAIL
    assert_selector "#account_menu:popover-open"

    within("#account_menu") do
      assert_text EMAIL
      assert_selector "[role=menuitem]", count: 1
    end
  end

  # Pelo ARIA, menu só contém menuitem, group e separator: o "Signed in as" fica fora dele e o descreve.
  test "the signed in text sits outside the menu and describes it" do
    click_on EMAIL
    assert_selector "#account_menu:popover-open"

    assert_no_selector "[role=menu] p", visible: :all
    menu = find("#account_menu [role=menu]")
    assert_equal [ "menuitem" ], menu.all("*").filter_map { it[:role] }.uniq
    assert_includes find_by_id(menu["aria-describedby"]).text, EMAIL
  end

  test "log out from the keyboard" do
    assert_selector "button[popovertarget=account_menu]"
    find("button[popovertarget=account_menu]").send_keys(:enter)
    assert_selector "[role=menuitem]:focus", text: "Log out"
    send_keys :end, :enter

    assert_text "Logged out."
  end
end
