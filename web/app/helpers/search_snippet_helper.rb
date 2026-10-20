module SearchSnippetHelper
  MARKED_TERM = /\u0002(.*?)\u0003/m
  STRAY_MARKER = /[\u0002\u0003]/

  # O trecho vem do texto cru da nota, nunca como HTML: cada parte é escapada e só o termo entre U+0002 e U+0003
  # vira <mark>. Marcador sem par some. O split com o grupo alterna texto (par) e termo casado (ímpar).
  def highlighted_snippet(snippet)
    parts = snippet.to_s.split(MARKED_TERM).map { it.gsub(STRAY_MARKER, "") }
    safe_join(parts.each_with_index.map { |part, index| index.odd? ? tag.mark(part) : part })
  end
end
