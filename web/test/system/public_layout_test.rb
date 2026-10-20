require "application_system_test_case"

# Telas sem sessão: o formulário fica no centro da tela, no desktop (ao lado do mascote) e no celular.
class PublicLayoutTest < ApplicationSystemTestCase
  CENTER_SCRIPT = <<~JS.freeze
    const rect = arguments[0].getBoundingClientRect()
    return [ rect.left + rect.width / 2 - window.innerWidth / 2, rect.top + rect.height / 2 - window.innerHeight / 2 ]
  JS

  setup { use_fake_api(notes: []) }

  test "the login sits in the middle of a desktop screen" do
    visit new_session_path

    assert_centered find("main"), axes: [ :vertical ]
  end

  test "the login sits in the middle of a phone screen" do
    page.current_window.resize_to(390, 844)
    visit new_session_path

    assert_centered find("main > div", text: "Log in"), axes: [ :horizontal, :vertical ]
  ensure
    page.current_window.resize_to(1400, 900)
  end

  # O dialogo de confirmacao do painel some com o layout dele: a tela publica volta ao confirm() do navegador, sem
  # chamar o showModal() de um dialogo fora da pagina.
  test "after logging out a turbo confirm goes back to the browser confirm" do
    sign_in_through_form
    click_on FakeMarkuppAccounts::DEFAULT_EMAIL
    click_on "Log out"
    assert_text "Logged out."

    execute_script("window.confirm = () => { window.nativeConfirmCalls = 1; return false }")
    execute_script("Turbo.config.forms.confirm('Delete?', document.createElement('form'))")

    assert_equal 1, evaluate_script("window.nativeConfirmCalls")
  end

  private
    def assert_centered(element, axes:, tolerance: 24)
      horizontal, vertical = evaluate_script("(function() { #{CENTER_SCRIPT} }).apply(null, arguments)", element)
      offsets = { horizontal:, vertical: }.slice(*axes)
      offsets.each do |axis, offset|
        assert_operator offset.abs, :<=, tolerance, "#{element.tag_name} is #{offset.round}px off the #{axis} center, expected at most #{tolerance}px"
      end
    end
end
