# Seção Agents, como a página de conectar do Linear ou do Notion: uma aba por cliente MCP, com o comando, o arquivo de
# configuração e o deeplink para esta instância. É de cada conta, então não pede tenant. O catálogo e a url pública da
# instância vêm do initializer agent_clients.rb; os testes trocam a url. Sem a url do MCP a instância não serve /mcp,
# e a página só avisa disso.
module Settings
  class AgentsController < BaseController
    class_attribute :agent_client_catalog, :agent_instance_url, :agent_mcp_url

    helper_method :agent_clients, :chosen_agent_client

    def show
      return render(:mcp_missing) if agent_mcp_url.blank?

      redirect_to settings_agents_path unless chosen_agent_client
    end

    private
      def agent_clients = @agent_clients ||= agent_client_catalog.clients_for(agent_instance_url, mcp_url: agent_mcp_url)

      # Sem client na url, a primeira aba; com um id que o catálogo não tem, nil, e o show volta para a primeira.
      def chosen_agent_client
        return agent_clients.first if params[:client].blank?

        agent_clients.find { it.id == params[:client] }
      end
  end
end
