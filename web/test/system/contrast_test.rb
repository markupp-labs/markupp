require "application_system_test_case"

# Contraste WCAG AA no tema claro (o do Chromium headless; o código confere também o escuro): 4.5:1 para texto comum, 3:1 para texto grande.
class ContrastTest < ApplicationSystemTestCase
  include ContrastAssertions

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

  test "the current item reads clearly in the tree, the settings and the activity bar" do
    visit settings_appearance_path
    within("main") { click_on "Light" }

    assert_readable find("main nav a[aria-current=page]")
    assert_readable find(".activity-bar-side a[aria-current=page]")
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
end
