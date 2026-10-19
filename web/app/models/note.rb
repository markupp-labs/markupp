class Note
  include ActiveModel::Model
  include ActiveModel::Attributes

  attribute :id, :string
  attribute :path, :string
  attribute :content, :string
  attribute :created_at, :datetime
  attribute :updated_at, :datetime

  SEARCH_LIMIT = 20
  UNTITLED_LIMIT = 100

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

  def self.from_api(attributes) = new(attributes.slice(*attribute_names))
  private_class_method :from_api

  def self.api_path(id) = MarkuppApi.path("notes", id)

  def self.untitled(folder) = new(path: [ folder.presence, "#{untitled_name}.md" ].compact.join("/"), content: "")

  def self.untitled_name = I18n.t("notes.untitled")

  MARKDOWN_EXTENSION = ".md"

  # O título é o nome do arquivo sem .md, como no Obsidian; título em branco deixa o nome vazio (update recusa).
  def self.compose_path(folder:, title:, markdown: true)
    [ folder.to_s.gsub(%r{\A/+|/+\z}, "").presence, "#{title.to_s.strip}#{MARKDOWN_EXTENSION if markdown}" ].compact.join("/")
  end

  def persisted? = id.present?

  # O navegador troca as quebras de linha da nota; ela volta com a que tinha.
  # ponytail: nota com quebras mistas volta com uma só, guardar a de cada linha se aparecer nota assim
  def self.restore_line_breaks(content, crlf:)
    lf = content.to_s.gsub("\r\n", "\n")
    crlf ? lf.gsub("\n", "\r\n") : lf
  end

  def crlf? = content.to_s.include?("\r\n")

  # Nota com o nome padrão ganha número quando o nome já existe; caminho escolhido pelo usuário não.
  def save(api)
    refusing_path do
      created = untitled? ? post_numbered(api) : post(api)
      assign_attributes(created.slice("id", "created_at", "updated_at"))
    end
  end

  # updated_at é a versão lida; a API recusa com Conflict se a nota mudou depois, a menos que force.
  # A API guarda a versão em milissegundos, então lastModifiedAt vai com essa precisão.
  # Título em branco não renomeia para o nome padrão: recusa sem chamar a API.
  def update(api, attributes, force: false)
    assign_attributes(attributes)
    return errors.add(:title, :blank) && false if title.empty?

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

  def untitled? = name == "#{Note.untitled_name}#{MARKDOWN_EXTENSION}"

  def folder_names = path_segments[...-1]

  private
    # A barra no fim fica como nome vazio ("work/" é título em branco na pasta work).
    def path_segments = path.to_s.squeeze("/").delete_prefix("/").split("/", -1)

    def post(api) = api.post("/notes", path:, content:)

    # Caminho recusado pela API vira erro em path e devolve false; a regra de caminho fica só no servidor.
    def refusing_path
      yield
      true
    rescue MarkuppApi::DuplicatePath
      errors.add(:path, :taken) && false
    rescue MarkuppApi::InvalidPath
      errors.add(:path, :invalid) && false
    end

    def post_numbered(api)
      (0..UNTITLED_LIMIT).each do |number|
        self.path = untitled_path(number)
        return post(api)
      rescue MarkuppApi::DuplicatePath
        next
      end
      raise MarkuppApi::DuplicatePath, "POST /notes found #{path} taken, expected a free untitled name within #{UNTITLED_LIMIT} numbered tries"
    end

    def untitled_path(number)
      name = [ Note.untitled_name, number.nonzero? ].compact.join(" ")
      [ *folder_names, "#{name}.md" ].join("/")
    end
end
