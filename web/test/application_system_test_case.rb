require "test_helper"

# Não há Chrome no PATH: o selenium-manager usa o Chromium que o Playwright baixou e acha o chromedriver da mesma versão.
ENV["SE_BROWSER_PATH"] ||= File.expand_path("~/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome")

class ApplicationSystemTestCase < ActionDispatch::SystemTestCase
  # HEADED=1 abre a janela do navegador para acompanhar o teste.
  driven_by :selenium, using: ENV["HEADED"] ? :chrome : :headless_chrome, screen_size: [ 1400, 900 ]
end
