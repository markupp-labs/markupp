require "application_system_test_case"

# Contraste WCAG AA no tema claro (o do Chromium headless; o código confere também o escuro): 4.5:1 para texto comum, 3:1 para texto grande.
class ContrastTest < ApplicationSystemTestCase
  # Cor de qualquer formato (light-dark, color-mix, oklab) vira rgba pelo canvas; o fundo é o do primeiro
  # ancestral com fundo, e o texto translúcido é misturado sobre ele antes da conta.
  CONTRAST_SCRIPT = <<~JS.freeze
    const [element, pseudo] = arguments
    const context = document.createElement("canvas").getContext("2d")
    const rgba = (color) => { context.clearRect(0, 0, 1, 1); context.fillStyle = color; context.fillRect(0, 0, 1, 1); return [ ...context.getImageData(0, 0, 1, 1).data ] }
    let node = element, back = rgba(getComputedStyle(node).backgroundColor)
    while (back[3] == 0 && node.parentElement) { node = node.parentElement; back = rgba(getComputedStyle(node).backgroundColor) }
    const front = rgba(getComputedStyle(element, pseudo).color)
    const blended = front.slice(0, 3).map((value, index) => value * front[3] / 255 + back[index] * (1 - front[3] / 255))
    const luminance = (rgb) => rgb.map((value) => { const c = value / 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4 })
                                  .reduce((sum, c, index) => sum + c * [ 0.2126, 0.7152, 0.0722 ][index], 0)
    const [ light, dark ] = [ luminance(blended), luminance(back.slice(0, 3)) ].sort((a, b) => b - a)
    return (light + 0.05) / (dark + 0.05)
  JS

  # Um bloco por família de token do tema: comentário, classe, título, string, número, símbolo, literal, variável, diff.
  CODE_NOTE = <<~MARKDOWN.freeze
    ```ruby
    # Greets someone
    class Greeter < Base
      def greet(name = nil)
        puts "Hi \#{name}", 42, :ok, true, @count
      end
    end
    ```

    ```diff
    - old
    + new
    ```
  MARKDOWN

  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" }, { id: "2", path: "code.md", content: CODE_NOTE } ])
    sign_in_through_form
  end

  test "the current item reads clearly in the tree, the settings and the sidebar footer" do
    visit settings_appearance_path
    within("main") { click_on "Light" }

    assert_readable find("main nav a[aria-current=page]")
    assert_readable find("footer a[aria-current=page]")
    assert_readable find("main button[aria-pressed=true]", text: "Light")

    visit note_path("1")
    assert_readable find("#sidebar a[aria-current=page]")
  end

  test "highlighted code reads clearly in the light and the dark theme" do
    %w[Light Dark].each do |theme|
      visit settings_appearance_path
      within("main") { click_on theme }
      # O tema troca por um PATCH que volta para Aparência; abrir a nota antes disso perde a corrida para o redirect.
      assert_selector "main button[aria-pressed=true]", text: theme
      visit note_path("2")
      click_on "Read"
      assert_code_tokens_readable
    end
  end

  # O placeholder é o único rótulo visível da paleta e da busca de tenants: o padrão do Tailwind (ink a 50%) não chega ao AA.
  test "the placeholders of the palette and the tenant filter read clearly in the light and the dark theme" do
    %w[Light Dark].each do |theme|
      visit settings_appearance_path
      within("main") { click_on theme }
      assert_selector "main button[aria-pressed=true]", text: theme
      visit tenants_path
      assert_readable find("input[type=search][aria-label='Filter tenants']"), pseudo: "::placeholder"
      send_keys [ :control, :shift, "p" ]
      assert_readable find("dialog#command_palette[open] input"), pseudo: "::placeholder"
    end
  end

  private
    def assert_code_tokens_readable
      assert_selector ".note-preview pre code"
      tokens = all(".note-preview pre code span[class*=hljs-]")
      assert_not_empty tokens, "expected highlighted tokens in the code blocks, found none"
      tokens.each { |token| assert_readable token }
    end

    def assert_readable(element, pseudo: nil, minimum: 4.5)
      ratio = evaluate_script("(function() { #{CONTRAST_SCRIPT} }).apply(null, arguments)", element, pseudo)
      assert_operator ratio, :>=, minimum, "contrast of #{element.tag_name} #{element.text.inspect}#{pseudo} is #{ratio.round(2)}:1, expected at least #{minimum}:1"
    end
end
