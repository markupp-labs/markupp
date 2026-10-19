require "application_system_test_case"

# Rodapé da barra lateral numa linha: o email abre o menu da conta (Account, Settings, Log out), à direita só os ícones
# de tema e configurações. O menu anda pelo teclado como o menu de contexto da árvore.
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

  test "the email opens the menu with the focus on the first item, j and k move and esc gives the focus back" do
    click_on EMAIL

    assert_selector "#account_menu[role=menu]:popover-open"
    assert_selector "[role=menuitem]:focus", text: "Account"
    send_keys "j"
    assert_selector "[role=menuitem]:focus", text: "Settings"
    send_keys "k", "k"
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

  test "the menu items lead to the account and the settings" do
    click_on EMAIL
    assert_selector "#account_menu:popover-open"
    within("#account_menu") { click_on "Account" }

    assert_selector "main h1", text: "Account"
  end

  test "log out from the keyboard" do
    assert_selector "button[popovertarget=account_menu]"
    find("button[popovertarget=account_menu]").send_keys(:enter)
    send_keys :end, :enter

    assert_text "Logged out."
  end
end
