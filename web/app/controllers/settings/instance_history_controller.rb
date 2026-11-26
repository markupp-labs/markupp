# Seção Histórico do grupo Instance (ADR-0061): os acessos e as mudanças sensíveis, do mais novo para trás, com filtros e
# páginas. Sem eventos, diz até quando o histórico vai.
module Settings
  class InstanceHistoryController < InstanceAdminController
    # event é a ação do evento: action o Rails reserva para o nome da ação do controller.
    FILTERS = %i[account event result ip since until].freeze

    def show
      @filters = params.permit(*FILTERS).to_h.symbolize_keys
      @offset = params[:offset].to_i.clamp(0, V1Listing::MAX_OFFSET)
      @events = AuditEvent.page(credential.api, @filters, offset: @offset)
      @retention = InstanceRetention.fetch(credential.api) if @events.empty?
    end
  end
end
