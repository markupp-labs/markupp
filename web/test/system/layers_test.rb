require "application_system_test_case"

# Camadas como as layers do Hyprland: menus e diálogos no surface (mais claro que a barra lateral, surface-sunken), com
# a borda line e uma sombra tingida da paleta (nunca o preto puro); atrás dos diálogos modais o fundo escurece de
# verdade (--modal-scrim: o ink do lotus a 35% no claro, o --scrim da gaveta do celular no escuro). O texto de cada
# camada passa do AA nos dois temas.
class LayersTest < ApplicationSystemTestCase
  include ContrastAssertions

  # Cor computada de um token do tema, pelo mesmo caminho do CSS (light-dark resolvido pelo color-scheme).
  TOKEN_COLOR = <<~JS.freeze
    const probe = document.createElement("i")
    probe.style.color = `var(${arguments[0]})`
    document.body.append(probe)
    const color = getComputedStyle(probe).color
    probe.remove()
    return color
  JS

  # Cada elemento visível da camada com texto próprio, para a conta de contraste.
  TEXT_ELEMENTS = <<~JS.freeze
    return [ arguments[0], ...arguments[0].querySelectorAll("*") ].filter((element) =>
      element.checkVisibility() && [ ...element.childNodes ].some((node) => node.nodeType == Node.TEXT_NODE && node.textContent.trim()))
  JS

  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form
  end

  test "menus and dialogs float on the surface with a tinted shadow, over a darkened backdrop, readable in both themes" do
    %w[Light Dark].each do |theme|
      @theme = theme
      choose_theme theme
      visit note_path("1")
      assert_field "Path", with: "/plan"

      assert_layer("#tree_menu") { find("[role=treeitem]", text: "plan").right_click }
      assert_layer("#account_menu") { click_on FakeMarkuppAccounts::DEFAULT_EMAIL }
      assert_layer("dialog#command_palette", modal: true) { send_keys [ :control, :shift, "p" ] }
      assert_layer("dialog[data-search-target=dialog]", modal: true) { send_keys [ :control, "k" ] }
      assert_layer("dialog#shortcut_help", modal: true) { open_help }
      assert_layer("dialog#confirm_dialog", modal: true) { click_on "Delete" }
      visit new_tenant_path
      assert_layer("dialog#new_tenant", modal: true)
    end
  end

  # As teclas da ajuda no ink: o accent sobre o fundo do diálogo ficava no limite do AA no claro.
  test "the keys of the help use the text color" do
    visit note_path("1")
    open_help
    ink = token_color("--ink")

    colors = evaluate_script("[ ...document.querySelectorAll('#shortcut_help dt kbd') ].map((kbd) => getComputedStyle(kbd).color)").uniq
    assert_equal [ ink ], colors
  end

  private
    def choose_theme(theme)
      visit settings_appearance_path
      within("main") { click_on theme }
      assert_selector "main button[aria-pressed=true]", text: theme
    end

    def open_help
      find("[role=treeitem]", text: "plan").execute_script("this.focus()")
      press_through_cdp("?", "Slash", keycode: 191, modifiers: 8)
    end

    def token_color(token) = evaluate_script("(function() { #{TOKEN_COLOR} }).apply(null, arguments)", token)

    def assert_layer(selector, modal: false)
      yield if block_given?
      layer = find("#{selector}:is(:popover-open, [open])")
      assert_floating layer, modal
      evaluate_script("(function() { #{TEXT_ELEMENTS} }).apply(null, arguments)", layer).each { assert_readable it }
      send_keys :escape
      assert_no_selector "#{selector}:is(:popover-open, [open])"
    end

    def assert_floating(layer, modal)
      style = layer.evaluate_script("(() => { const style = getComputedStyle(this); return [ style.backgroundColor, style.borderTopColor, style.boxShadow, getComputedStyle(this, '::backdrop').backgroundColor ] })()")
      background, border, shadow, backdrop = style

      assert_equal token_color("--surface"), background, "#{layer[:id]} should sit on the surface"
      assert_equal token_color("--line"), border, "#{layer[:id]} should have the line border"
      assert_match(/rgba\(/, shadow, "#{layer[:id]} should have a soft shadow")
      assert_no_match(/rgba?\(0, 0, 0[,)]/, shadow, "#{layer[:id]} shadow should be tinted, not pure black")
      assert_equal modal_scrim, backdrop, "#{layer[:id]} should darken what is behind it" if modal
    end

    # O ink do lotus a 35% no claro; no escuro, o mesmo fundo da gaveta do celular.
    def modal_scrim = @theme == "Light" ? "rgba(84, 84, 100, 0.35)" : token_color("--scrim")
end
