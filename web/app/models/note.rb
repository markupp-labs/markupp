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

  def persisted? = id.present?

  # updated_at é a versão lida; a API recusa com Conflict se a nota mudou depois, a menos que force.
  # A API guarda a versão em milissegundos, então lastModifiedAt vai com essa precisão.
  def update(attributes, force: false)
    assign_attributes(attributes)
    api.put(Note.api_path(id), path:, content:, lastModifiedAt: updated_at&.iso8601(3), force:)
  end

  def name = path_segments.last

  def folder_names = path_segments[...-1]

  private
    def path_segments = path.split("/").reject(&:empty?)
end
