# Visão Links da barra lateral, como os painéis Backlinks e Outgoing links do Obsidian: o frame note_links carrega
# quando a visão aparece, com as ligações para a nota aberta, as que ela faz, as notas que citam o título dela sem ligar
# e as sugestões de ligação. A página de cada nota pede o frame de novo, então trocar de nota e salvar atualizam.
class NoteLinksController < ApplicationController
  before_action :require_tenant

  def show
    api = credential.api
    @sections = NoteLinkSection::NAMES.index_with { NoteLinkSection.page(api, params[:note_id], it) }
  end
end
