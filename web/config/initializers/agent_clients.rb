# Página Agents: o catálogo de clientes MCP (cópia gerada pelo npm run clients:sync em mcp/) e a url pública da
# instância, a mesma MARKUPP_PUBLIC_URL que a API anuncia como issuer. MARKUPP_MCP_URL só vem quando a instância serve
# o MCP; sem ela a página avisa que o MCP está desligado.
Rails.application.config.to_prepare do
  Settings::AgentsController.agent_client_catalog = AgentClientCatalog.load(Rails.root.join("config/agent_clients.json"))
  Settings::AgentsController.agent_instance_url = ENV.fetch("MARKUPP_PUBLIC_URL", "http://localhost:8080")
  Settings::AgentsController.agent_mcp_url = ENV["MARKUPP_MCP_URL"].presence
end
