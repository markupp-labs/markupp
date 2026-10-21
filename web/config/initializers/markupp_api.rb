Rails.application.config.to_prepare do
  url = ENV.fetch("MARKUPP_API_URL", "http://localhost:8080")
  ApplicationController.markupp_api_factory = ->(**credentials) { MarkuppApi.new(url, **credentials) }
end
