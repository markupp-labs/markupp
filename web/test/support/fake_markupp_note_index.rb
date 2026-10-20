# Rotas de leitura de notas sob /v1 da FakeMarkuppApi, sobre as notas da FakeMarkuppNotes: as do cofre, com filtros e
# ordem, e a busca ranqueada. A organização derivada (degree, pagerank, community) é a que a nota do cenário traz.
class FakeMarkuppNoteIndex
  SORT_KEYS = { "pagerank" => "pagerank", "degree" => "degree", "updated" => "updated_at" }.freeze
  VAULT_NOTES = %r{\A/v1/vaults/(?<vault_id>[^/?]+)/notes(?:\?|\z)}

  def initialize(notes) = @notes = notes

  def get(path, **)
    params = FakeMarkuppPaging.query(path)
    return search(params) if path.start_with?("/v1/search/notes?")

    vault_id = VAULT_NOTES.match(path)&.[](:vault_id) or raise ArgumentError, "fake API has no route for GET #{path}, expected /v1/vaults/{id}/notes"
    FakeMarkuppPaging.page(sorted(filtered(in_vault(vault_id), params), params), params).map { list_item(it) }
  end

  private
    def in_vault(vault_id) = @notes.notes.select { it["vault_id"] == vault_id }

    # Como a API: orphan=false inclui a nota que o indexador ainda não viu (degree nil).
    def filtered(notes, params)
      notes = notes.select { (it["degree"] == 0) == (params["orphan"] == "true") } if params.key?("orphan")
      notes = notes.select { it["community"].to_s == params["community"] } if params.key?("community")
      notes
    end

    # Sem sort, por caminho; a nota sem o valor vai para o fim nos dois sentidos. Empate segue pelo caminho crescente,
    # nos dois sentidos, como a API.
    def sorted(notes, params)
      key = SORT_KEYS[params["sort"]] or return notes.sort_by { it["path"] }
      ranked, unranked = notes.partition { it[key] }
      sign = params["direction"] == "asc" ? 1 : -1
      ranked.sort { |a, b| (sign * (a[key] <=> b[key])).nonzero? || a["path"] <=> b["path"] } + unranked.sort_by { it["path"] }
    end

    def list_item(note) = summary(note).merge(note.slice("updated_at", "degree", "pagerank", "community"))

    def summary(note) = { "id" => note["id"], "title" => File.basename(note["path"], ".md"), "path" => note["path"] }

    # Simplificação da busca da API: todos os termos no caminho ou no conteúdo, sem diferenciar caixa. O trecho é o
    # conteúdo com cada termo entre U+0002 e U+0003, e rank conta as ocorrências.
    def search(params)
      terms = params.fetch("q").downcase.split
      return [] if terms.empty?

      pattern = Regexp.new(terms.map { Regexp.escape(it) }.join("|"), Regexp::IGNORECASE)
      matches = @notes.notes.select { |note| matches?(note, terms, params["vault"]) }
      FakeMarkuppPaging.page(matches.map { hit(it, pattern) }.sort_by { -it["rank"] }, params)
    end

    def matches?(note, terms, vault)
      text = "#{note['path']} #{note['content']}".downcase
      (vault.nil? || note["vault_id"] == vault) && terms.all? { text.include?(it) }
    end

    def hit(note, pattern)
      content = note["content"].to_s
      { "note" => summary(note), "vault_id" => note["vault_id"], "rank" => content.scan(pattern).size.to_f,
        "snippet" => content.gsub(pattern) { "\u0002#{it}\u0003" } }
    end
end
