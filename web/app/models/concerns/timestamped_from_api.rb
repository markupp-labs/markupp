# Monta o Data a partir do json da API: só os campos dele, com o campo de data que sempre vem já como Time.
module TimestampedFromApi
  def timestamped_from(attributes, time_field)
    fields = attributes.symbolize_keys.slice(*members)
    new(**fields, time_field => Time.zone.parse(fields.fetch(time_field)))
  end
end
