require "test_helper"

# Configurações, seção Agents: como registrar o servidor MCP desta instância em cada cliente, para todo mundo logado.
module Settings
  class AgentsControllerTest < ActionDispatch::IntegrationTest
    INSTANCE_URL = "https://notas.exemplo.com.br".freeze

    setup do
      @original_urls = [ AgentsController.agent_instance_url, AgentsController.agent_mcp_url ]
      AgentsController.agent_instance_url = INSTANCE_URL
      AgentsController.agent_mcp_url = "#{INSTANCE_URL}/mcp"
    end

    teardown { AgentsController.agent_instance_url, AgentsController.agent_mcp_url = @original_urls }

    test "show asks for login first" do
      get settings_agents_path

      assert_redirected_to new_session_path
    end

    test "show opens on Claude Code with the command for this instance" do
      use_fake_api(tenants: AccountsControllerTest::MEMBER_ONLY)
      sign_in_as

      get settings_agents_path

      assert_response :success
      assert_select "main h1", "Agents"
      assert_select "main nav a[href='/settings/agents'][aria-current=page]", text: "Agents"
      assert_select "main [aria-label='MCP clients'] a[aria-current=page]", text: "Claude Code"
      assert_select "main pre", text: "claude mcp add --transport http markupp #{INSTANCE_URL}/mcp"
      assert_select "main button[data-action='snippet-copy#copy']", text: "Copy"
    end

    test "show has a tab for each client of the catalog" do
      use_fake_api(tenants: [])
      sign_in_as

      get settings_agents_path

      [ "Claude Code", "Codex", "Cursor", "VS Code", "Claude Desktop", "Gemini CLI", "Windsurf", "stdio (npx)" ].each do |name|
        assert_select "main [aria-label='MCP clients'] a", text: name
      end
    end

    test "the Cursor tab has the install deeplink and the config file" do
      use_fake_api(tenants: [])
      sign_in_as

      get settings_agents_path(client: "cursor")

      assert_select "main [aria-label='MCP clients'] a[aria-current=page]", text: "Cursor"
      assert_select "main a[href^='cursor://anysphere.cursor-deeplink/mcp/install?name=markupp&config=']", text: "Add to Cursor"
      assert_select "main", text: %r{~/.cursor/mcp.json}
      assert_select "main a[href='https://cursor.com/docs/context/mcp']"
    end

    test "the VS Code tab has the install deeplink" do
      use_fake_api(tenants: [])
      sign_in_as

      get settings_agents_path(client: "vscode")

      assert_select "main a[href^='vscode:mcp/install?']", text: "Add to VS Code"
    end

    # Sem MARKUPP_MCP_URL a instância não serve /mcp: a página avisa em vez de mostrar comandos que dariam 404.
    test "show warns that the MCP is off when the instance has no MCP url" do
      AgentsController.agent_mcp_url = nil
      use_fake_api(tenants: [])
      sign_in_as

      get settings_agents_path(client: "cursor")

      assert_response :success
      assert_select "main h1", "Agents"
      assert_select "main [role=note]", text: /This instance has no MCP server/
      assert_select "main pre", count: 0
      assert_select "main [aria-label='MCP clients']", count: 0
      assert_select "main a[href^='cursor://']", count: 0
    end

    test "an unknown client goes back to the first tab" do
      use_fake_api(tenants: [])
      sign_in_as

      get settings_agents_path(client: "emacs")

      assert_redirected_to settings_agents_path
    end
  end
end
