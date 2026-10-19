require "test_helper"

# Não há Chrome no PATH: o selenium-manager usa o Chromium que o Playwright baixou e acha o chromedriver da mesma versão.
ENV["SE_BROWSER_PATH"] ||= File.expand_path("~/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome")

# Campos sem <label> visível (como a busca) são achados pelo aria-label, como um leitor de tela acharia.
Capybara.enable_aria_label = true

class ApplicationSystemTestCase < ActionDispatch::SystemTestCase
  # HEADED=1 abre a janela do navegador para acompanhar o teste.
  driven_by :selenium, using: ENV["HEADED"] ? :chrome : :headless_chrome, screen_size: [ 1400, 900 ]

  # Entra pelo formulário de login, contra a fake, e espera a barra lateral com o email.
  def sign_in_through_form(email = FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD)
    visit new_session_path
    fill_in "Email", with: email
    fill_in "Password", with: password
    click_on "Log in"
    # O login cruza login, /auth/me e /me/tenants; com um Chrome por núcleo em paralelo passa dos 2s padrão.
    assert_selector "nav", text: email, wait: 10
  end
end
