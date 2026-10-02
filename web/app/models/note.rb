class Note
  include ActiveModel::Model
  include ActiveModel::Attributes

  attribute :id, :string
  attribute :path, :string
  attribute :content, :string
  attribute :created_at, :datetime
  attribute :updated_at, :datetime

  SEARCH_LIMIT = 20

  class_attribute :api

  # ponytail: GET /notes traz o conteúdo de todas as notas, paginar quando a API paginar
  def self.all
    api.get("/notes").map { |attributes| from_api(attributes) }
  end

  def self.find(id) = from_api(api.get(api_path(id)))

  # Os resultados não trazem o conteúdo, só id, path e updated_at.
  def self.search(query)
    return [] if query.blank?

    api.get("/notes/search?#{{ query:, limit: SEARCH_LIMIT }.to_query}").map { |attributes| from_api(attributes) }
  end

  def self.from_api(attributes) = new(attributes.slice(*attribute_names))

  def self.api_path(id) = "/notes/#{ERB::Util.url_encode(id)}"
  private_class_method :from_api

  UNTITLED_LIMIT = 100

  def self.untitled(folder) = new(path: [ folder.presence, "#{untitled_name}.md" ].compact.join("/"), content: "")

  def self.untitled_name = I18n.t("notes.untitled")

  def persisted? = id.present?

  # Nota com o nome padrão ganha número quando o nome já existe; caminho escolhido pelo usuário não.
  def save
    created = untitled? ? post_numbered : post
    assign_attributes(created.slice("id", "created_at", "updated_at"))
  end

  # updated_at é a versão lida; a API recusa com Conflict se a nota mudou depois, a menos que force.
  # A API guarda a versão em milissegundos, então lastModifiedAt vai com essa precisão.
  # Caminho recusado pela API vira erro em path e devolve false; a regra de caminho fica só no servidor.
  def update(attributes, force: false)
    assign_attributes(attributes)
    api.put(Note.api_path(id), path:, content:, lastModifiedAt: updated_at&.iso8601(3), force:)
  rescue MarkuppApi::DuplicatePath
    errors.add(:path, :taken) && false
  rescue MarkuppApi::InvalidPath
    errors.add(:path, :invalid) && false
  end

  # Já apagada (404) conta como apagada.
  def destroy
    api.delete(Note.api_path(id))
  rescue MarkuppApi::NotFound
    nil
  end

  def name = path_segments.last

  def folder_names = path_segments[...-1]

  private
    def path_segments = path.split("/").reject(&:empty?)

    def post = api.post("/notes", path:, content:)

    def untitled? = name == "#{Note.untitled_name}.md"

    def post_numbered
      (0..UNTITLED_LIMIT).each do |number|
        self.path = untitled_path(number)
        return post
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
