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
    api.get("/notes").map { |attributes| new(attributes.slice(*attribute_names)) }
  end

  def name = path_segments.last

  def folder_names = path_segments[...-1]

  private
    def path_segments = path.split("/").reject(&:empty?)
end
