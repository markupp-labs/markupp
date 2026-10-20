# Query e paginação das listas da FakeMarkuppApi sob /v1, como a API: offset 0 e limit 10 quando não vêm.
module FakeMarkuppPaging
  def self.query(path) = URI.decode_www_form(URI(path).query.to_s).to_h

  def self.page(items, params) = items.drop(Integer(params.fetch("offset", 0))).first(Integer(params.fetch("limit", 10)))
end
