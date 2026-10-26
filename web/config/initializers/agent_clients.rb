# Página Agents: o catálogo de clientes MCP (cópia gerada pelo npm run clients:sync em mcp/) e a url pública da
# instância, a mesma MARKUPP_PUBLIC_URL que a API anuncia como issuer; o MCP fica nela em /mcp.
Rails.application.config.to_prepare do
  Settings::AgentsController.agent_client_catalog = AgentClientCatalog.load(Rails.root.join("config/agent_clients.json"))
  Settings::AgentsController.agent_instance_url = ENV.fetch("MARKUPP_PUBLIC_URL", "http://localhost:8080")
end
