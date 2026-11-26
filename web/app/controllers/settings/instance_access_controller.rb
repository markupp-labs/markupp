# Seção Acessos do grupo Instance (ADR-0061): os IPs e faixas bloqueados e o prazo de guarda do histórico e do IP.
module Settings
  class InstanceAccessController < InstanceAdminController
    def show
      @ip_blocks = IpBlock.all(credential.api)
      @retention = InstanceRetention.fetch(credential.api)
    end
  end
end
