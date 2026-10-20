require "test_helper"

# Não há Chrome no PATH: o selenium-manager usa o Chromium que o Playwright baixou e acha o chromedriver da mesma versão.
ENV["SE_BROWSER_PATH"] ||= File.expand_path("~/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome")

# Campos sem <label> visível (como a busca) são achados pelo aria-label, como um leitor de tela acharia.
Capybara.enable_aria_label = true

# Espera por condição até 6s, não os 2s padrão: com um Chrome por núcleo em paralelo, uma visita do Turbo leva mais de
# 2s entre a tecla e o render (Alt+N medido em 2,1s; o apagar da nota idem), e o teste olhava a página velha. O foco e o
# resto já estavam certos quando o render chegava. Passando, nada espera a mais: o Capybara para assim que acha.
Capybara.default_max_wait_time = 6

class ApplicationSystemTestCase < ActionDispatch::SystemTestCase
  # Um Chrome a cada dois núcleos: com um por núcleo a máquina satura e as esperas por condição estouram. O
  # parallelize vale para o processo todo, mas este arquivo só é carregado pelos testes de sistema; os unitários
  # seguem com todos os núcleos. PARALLEL_WORKERS, quando definido, ganha deste número (o próprio Rails o lê).
  parallelize(workers: (Etc.nprocessors / 2).clamp(1..))

  # HEADED=1 abre a janela do navegador para acompanhar o teste.
  driven_by :selenium, using: ENV["HEADED"] ? :chrome : :headless_chrome, screen_size: [ 1400, 900 ]

  # Sem animação por padrão: a troca é instantânea e quem mede posição ou estado não pega o meio de uma transição.
  # O teste de movimento (motion_test.rb) volta ao no-preference.
  setup { emulate_motion_preference("reduce") }

  def emulate_motion_preference(value)
    page.driver.browser.execute_cdp("Emulation.setEmulatedMedia", features: [ { name: "prefers-reduced-motion", value: } ])
  end

  # Tecla de pontuação pelo CDP: o chromedriver manda a pontuação com key vazio. modifiers 8 é o Shift.
  def press_through_cdp(key, code, keycode:, modifiers: 0)
    %w[keyDown keyUp].each do |type|
      page.driver.browser.execute_cdp("Input.dispatchKeyEvent", type:, key:, code:, text: key, modifiers:, windowsVirtualKeyCode: keycode)
    end
  end

  # Responde ao diálogo de confirmação do painel (layouts/_confirm_dialog), que toma o lugar do confirm() do navegador
  # no data-turbo-confirm: a ação roda, o diálogo abre com a pergunta, o botão escolhido responde e o diálogo fecha.
  def accept_panel_confirm(message, button: "Delete", &) = answer_panel_confirm(message, button, &)

  def dismiss_panel_confirm(message, button: "Cancel", &) = answer_panel_confirm(message, button, &)

  def answer_panel_confirm(message, button)
    yield
    within("dialog#confirm_dialog[open]") do
      assert_text message
      click_on button
    end
    assert_no_selector "dialog#confirm_dialog[open]"
  end

  # Entra pelo formulário de login, contra a fake, e espera a barra lateral com o email.
  def sign_in_through_form(email = FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD)
    visit new_session_path
    fill_in "Email", with: email
    fill_in "Password", with: password
    click_on "Log in"
    # O login cruza login, /auth/me e /me/tenants; com um Chrome por núcleo em paralelo passa dos 2s padrão.
    assert_selector "nav", text: email, wait: 10
  end

  def edge(element, side) = evaluate_script("arguments[0].getBoundingClientRect().#{side}", element)

  # Cor computada de um valor CSS (um token como var(--accent)), para comparar com a de um elemento.
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
end
