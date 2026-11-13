# Seção General do cofre ativo: o nome, que quem escreve renomeia, e a Danger zone que apaga o cofre (VaultsController).
module Settings
  class VaultGeneralController < VaultBaseController
    helper_method :vault_publication

    def show
    end

    private
      # A publicação do cofre inteiro, ou nil; só owner e admin a veem aqui, que são quem publica o cofre.
      def vault_publication = Publication.all(credential.api, vault_id: current_vault.id).find { it.kind == "vault" }
  end
end
