class Note
  include ActiveModel::Model
  include ActiveModel::Attributes

  attribute :id, :string
  attribute :path, :string
  attribute :content, :string
  attribute :created_at, :datetime
  attribute :updated_at, :datetime

  class_attribute :api

  # ponytail: GET /notes traz o conteúdo de todas as notas, paginar quando a API paginar
  def self.all
    api.get("/notes").map { |attributes| from_api(attributes) }
  end

  def self.find(id) = from_api(api.get("/notes/#{ERB::Util.url_encode(id)}"))

  def self.from_api(attributes) = new(attributes.slice(*attribute_names))
  private_class_method :from_api

  def persisted? = id.present?

  def name = path_segments.last

  def folder_names = path_segments[...-1]

  private
    def path_segments = path.split("/").reject(&:empty?)
end
