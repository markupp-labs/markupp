# Visão Links da barra lateral, como os painéis Backlinks e Outgoing links do Obsidian: o frame note_links carrega
# quando a visão aparece, com as ligações da nota aberta.
class NoteLinksController < ApplicationController
  before_action :require_tenant

  def show; end
end
