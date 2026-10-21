# Visão Links da barra lateral (note_links/show): o frame que a página da nota pede, as seções recolhíveis e o tipo de
# cada ligação.
module NoteLinksHelper
  # Frame da nota aberta, que só carrega quando a visão aparece (lazy); a espera (o aria-busy que o Turbo põe no frame)
  # mostra o busy_spinner depois de 300ms, e nada enquanto a visão está escondida e o frame nem pediu.
  # O controller fica no frame, que o Turbo mantém: só o conteúdo dele troca.
  def note_links_frame(note)
    turbo_frame_tag "note_links", src: note_links_path(note), loading: :lazy, target: "_top",
                    data: { controller: "note-links", action: "keydown->note-links#navigate turbo:frame-load->note-links#frameLoaded",
                            note_links_copied_value: t("notes.sidebar.copied"), note_links_copy_failed_value: t("notes.form.copy_failed") } do
      busy_spinner(busy_when: "#note_links[aria-busy]", class: "px-2")
    end
  end

  # Seção recolhível, aberta, com o nome e a contagem no summary; vazia, diz que não há nada.
  def links_section(name, items, &)
    tag.details(id: "links_#{name}", open: true, class: "links-section") do
      safe_join([ links_section_summary(name, links_count(items, 0)),
                  items.empty? ? tag.p(t("note_links.none.#{name}"), class: "links-none") : tag.ul(class: "links-list", &) ])
    end
  end

  # Contagem da seção até a página que começa em offset: a API não dá o total, então toda página cheia diz só que há mais
  # (100+, sem subir para 200+) e a que não enche dá o número real.
  def links_count(items, offset)
    items.size == NoteLinkSection::PAGE_SIZE ? "#{NoteLinkSection::PAGE_SIZE}+" : (offset + items.size).to_s
  end

  # Frame de uma página da seção a partir de offset: o Show more da página anterior e, depois de carregar, as linhas.
  def links_page_frame_id(section, offset) = "links_#{section}_#{offset}"

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
