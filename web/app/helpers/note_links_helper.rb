# Visão Links da barra lateral (note_links/show): o frame que a página da nota pede, as seções recolhíveis e o tipo de
# cada ligação.
module NoteLinksHelper
  # Frame da nota aberta, que só carrega quando a visão aparece (lazy); a espera (o aria-busy que o Turbo põe no frame)
  # mostra o busy_spinner depois de 300ms, e nada enquanto a visão está escondida e o frame nem pediu.
  # O controller fica no frame, que o Turbo mantém: só o conteúdo dele troca.
  def note_links_frame(note)
    turbo_frame_tag "note_links", src: note_links_path(note), loading: :lazy, target: "_top",
                    data: { controller: "note-links", action: "keydown->note-links#navigate turbo:frame-load->note-links#focusWhenWaiting",
                            note_links_copied_value: t("notes.sidebar.copied"), note_links_copy_failed_value: t("notes.form.copy_failed") } do
      busy_spinner(busy_when: "#note_links[aria-busy]", class: "px-2")
    end
  end

  # Seção recolhível, aberta, com o nome e a contagem no summary; vazia, diz que não há nada.
  def links_section(name, items, &)
    tag.details(id: "links_#{name}", open: true, class: "links-section") do
      safe_join([ links_section_summary(name, items.size),
                  items.empty? ? tag.p(t("note_links.none.#{name}"), class: "links-none") : tag.ul(class: "links-list", &) ])
    end
  end

  # O que a linha mostra à direita: o predicado na ligação tipada, embed e inferred nas outras que não são o wikilink comum.
  def link_kind(link)
    link.type == "typed" ? link.predicate : link.type.presence_in(%w[embed inferred])
  end

  private
    def links_section_summary(name, count)
      tag.summary(class: "links-section-heading tree-row") do
        safe_join([ tag.span(t("note_links.sections.#{name}"), class: "links-section-title"), tag.span(count, class: "links-section-count") ])
      end
    end
end
