require "application_system_test_case"

# Página Agents: o botão Copy de cada trecho copia o texto do pre, sem entidades de HTML, e o status diz o resultado por
# 2 segundos (snippet_copy_controller.js).
class AgentsTest < ApplicationSystemTestCase
  INSTANCE_URL = "https://notas.exemplo.com.br".freeze
  CURSOR_CONFIG = "{\n  \"mcpServers\": {\n    \"markupp\": {\n      \"url\": \"#{INSTANCE_URL}/mcp\"\n    }\n  }\n}".freeze

  # Clipboard falso que recusa a escrita, como o navegador faz sem permissão.
  REJECTING_CLIPBOARD = <<~JS.freeze
    class RejectingClipboard { writeText() { return Promise.reject(new DOMException("denied", "NotAllowedError")) } }
    Object.defineProperty(navigator, "clipboard", { value: new RejectingClipboard(), configurable: true })
  JS

  setup do
    @original_urls = [ Settings::AgentsController.agent_instance_url, Settings::AgentsController.agent_mcp_url ]
    Settings::AgentsController.agent_instance_url = INSTANCE_URL
    Settings::AgentsController.agent_mcp_url = "#{INSTANCE_URL}/mcp"
    use_fake_api
    sign_in_through_form
    visit settings_agents_path(client: "cursor")
  end

  teardown { Settings::AgentsController.agent_instance_url, Settings::AgentsController.agent_mcp_url = @original_urls }

  test "copy puts the exact snippet, quotes included, in the clipboard and says so" do
    grant_clipboard
    within(cursor_config) do
      click_on "Copy"

      assert_selector "[role=status]", exact_text: "Copied"
    end
    assert_equal CURSOR_CONFIG, evaluate_async_script("navigator.clipboard.readText().then(arguments[0])")
  end

  test "copy says it could not copy when the clipboard refuses" do
    execute_script(REJECTING_CLIPBOARD)
    within(cursor_config) do
      click_on "Copy"

      assert_selector "[role=status]", exact_text: "Could not copy"
    end
  end

  test "the status clears after two seconds" do
    grant_clipboard
    within(cursor_config) do
      click_on "Copy"
      assert_selector "[role=status]", exact_text: "Copied"

      assert_selector "[role=status]", exact_text: "", visible: :all
    end
  end

  private
    def cursor_config = find("[data-controller=snippet-copy]", text: "Or add to ~/.cursor/mcp.json")
end
