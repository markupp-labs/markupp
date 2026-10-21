# Visão Links da barra lateral, como os painéis Backlinks e Outgoing links do Obsidian: o frame note_links carrega
# quando a visão aparece, com as ligações para a nota aberta, as que ela faz, as notas que citam o título dela sem ligar
# e as sugestões de ligação. A página de cada nota pede o frame de novo, então trocar de nota e salvar atualizam.
class NoteLinksController < ApplicationController
  # ponytail: uma página de 100 por seção (o máximo da API); paginar quando uma nota passar disso.
  LIST_LIMIT = 100

  before_action :require_tenant

  def show
    api = credential.api
    note_id = params[:note_id]
    @backlinks = Link.to_note(api, note_id, limit: LIST_LIMIT)
    @outgoing = Link.from_note(api, note_id, limit: LIST_LIMIT)
    @mentions = Mention.citing(api, note_id, limit: LIST_LIMIT)
    @suggestions = LinkSuggestion.for_note(api, note_id, limit: LIST_LIMIT)
  end
end
