Rails.application.config.to_prepare do
  Note.api = MarkuppApi.new(ENV.fetch("MARKUPP_API_URL", "http://localhost:8080"))
end
