ENV["RAILS_ENV"] ||= "test"
require_relative "../config/environment"
require "rails/test_help"
Dir[File.expand_path("support/*.rb", __dir__)].each { |file| require file }

module ActiveSupport
  class TestCase
    # Run tests in parallel with specified workers
    parallelize(workers: :number_of_processors)

    teardown { ApplicationController.markupp_api_factory = @original_factory if @original_factory }

    def use_fake_api(**options)
      @original_factory ||= ApplicationController.markupp_api_factory
      fake_api = FakeMarkuppApi.new(**options)
      ApplicationController.markupp_api_factory = ->(**) { fake_api }
      fake_api
    end
  end
end
