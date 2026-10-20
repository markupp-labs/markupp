class Note
  include ActiveModel::Model
  include ActiveModel::Attributes
  extend V1Listing

  attribute :id, :string
  attribute :path, :string
  attribute :content, :string
  attribute :created_at, :datetime
  attribute :updated_at, :datetime
  # O cofre da nota: é por ele que o painel chega às rotas do cofre (grafo, comunidades, notas do cofre).
  attribute :vault_id, :string
  # Organização derivada, que o indexador grava depois: nil na nota que ele ainda não viu.
  attribute :degree, :integer
  attribute :pagerank, :float
  attribute :community, :integer

  SEARCH_LIMIT = 20

  # ponytail: GET /notes traz o conteúdo de todas as notas, paginar quando a API paginar
  def self.all(api)
    api.get("/notes").map { from_api(it) }
  end

  def self.find(api, id) = from_api(api.get(api_path(id)))

  # Os resultados não trazem o conteúdo, só id, path e updated_at.
  def self.search(api, query)
    return [] if query.blank?

    api.get("/notes/search?#{{ query:, limit: SEARCH_LIMIT }.to_query}").map { from_api(it) }
  end

  # Notas do cofre sem o conteúdo, com a organização derivada; a query filtra (orphan, community) e ordena (sort, direction).
  def self.in_vault(api, vault_id, **query) = list_from(api, "vaults", vault_id, "notes", **query)

  # Também monta a nota resumida (id, título e caminho) que vem dentro de ligação, menção, comunidade e busca.
  def self.from_api(attributes) = new(attributes.slice(*attribute_names))

  def self.api_path(id) = MarkuppApi.path("notes", id)

  MARKDOWN_EXTENSION = ".md"

  def persisted? = id.present?

  # O navegador troca as quebras de linha da nota; ela volta com a que tinha.
  # ponytail: nota com quebras mistas volta com uma só, guardar a de cada linha se aparecer nota assim
  def self.restore_line_breaks(content, crlf:)
    lf = content.to_s.gsub("\r\n", "\n")
    crlf ? lf.gsub("\n", "\r\n") : lf
  end

  def crlf? = content.to_s.include?("\r\n")

  # Nota em branco no editor: nome vazio na pasta (folder vem como a barra lateral manda, com ou sem barras em volta).
  def self.blank(folder) = new(path: vim_normalized("#{folder}/"), content: "")

  # Não existe nome automático: título em branco recusa sem chamar a API.
  def save(api)
    return false if refusing_blank_title

    refusing_path { assign_attributes(post(api).slice("id", "created_at", "updated_at")) }
  end

  # updated_at é a versão lida; a API recusa com Conflict se a nota mudou depois, a menos que force.
  # A API guarda a versão em milissegundos, então lastModifiedAt vai com essa precisão.
  def update(api, attributes, force: false)
    assign_attributes(attributes)
    return false if refusing_blank_title

    refusing_path { api.put(Note.api_path(id), path:, content:, lastModifiedAt: updated_at&.iso8601(3), force:) }
  end

  # Já apagada (404) conta como apagada.
  def destroy(api)
    api.delete(Note.api_path(id))
  rescue MarkuppApi::NotFound
    nil
  end

  def name = path_segments.last

  def title = name.to_s.delete_suffix(MARKDOWN_EXTENSION)

  def folder = folder_names.join("/")

  def markdown? = name.to_s.end_with?(MARKDOWN_EXTENSION)

  def folder_names = path_segments[...-1]

  # Caminho digitado no campo da nota, como o :e do Vim com a pasta desta nota como diretório atual: barra no começo
  # parte da raiz. O .md final digitado é absorvido e volta uma vez só; outra extensão fica no nome (teste.pdf.md).
  # Nota sem .md continua sem enquanto o nome não muda. Barra no fim deixa o nome vazio (save e update recusam).
  def resolve(typed)
    typed = typed.to_s.strip
    base = Note.vim_normalized(typed.start_with?("/") ? typed : [ folder.presence, typed ].compact.join("/")).delete_suffix(MARKDOWN_EXTENSION)
    new_name = base.split("/", -1).last.to_s
    new_name.empty? || (!markdown? && new_name == name) ? base : "#{base}#{MARKDOWN_EXTENSION}"
  end

  # Como o Vim lê o caminho: "." é a pasta em que está e some, barras repetidas (e a do começo) valem uma. O nome no
  # fim fica, vazio com a barra ou o "." no fim. ".." vai como veio, para a API recusar com a mensagem dela.
  def self.vim_normalized(path)
    *folders, name = path.split("/", -1)
    [ *folders.reject { it.empty? || it == "." }, name == "." ? "" : name ].join("/")
  end

  # O caminho como o campo mostra: absoluto, sem .md.
  def location = "/#{path.to_s.delete_suffix(MARKDOWN_EXTENSION)}"

  private
    # A barra no fim fica como nome vazio ("work/" é título em branco na pasta work).
    def path_segments = Note.vim_normalized(path.to_s).split("/", -1)

    def post(api) = api.post("/notes", path:, content:)

    def refusing_blank_title = title.empty? && errors.add(:title, :blank)

    # Caminho recusado pela API vira erro em path e devolve false; a regra de caminho fica só no servidor.
    def refusing_path
      yield
      true
    rescue MarkuppApi::DuplicatePath
      errors.add(:path, :taken) && false
    rescue MarkuppApi::InvalidPath
      errors.add(:path, :invalid) && false
    end
end
