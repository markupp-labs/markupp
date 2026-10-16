ENV["RAILS_ENV"] ||= "test"
require_relative "../config/environment"
require "rails/test_help"
Dir[File.expand_path("support/*.rb", __dir__)].each { |file| require file }

module ActiveSupport
  class TestCase
    # Run tests in parallel with specified workers
    parallelize(workers: :number_of_processors)

    # Zera o contador do rate_limit do login, que mora no cache do controller.
    setup { ActionController::Base.cache_store.clear }
    teardown { ApplicationController.markupp_api_factory = @original_factory if @original_factory }

    def use_fake_api(**options)
      @original_factory ||= ApplicationController.markupp_api_factory
      fake_api = FakeMarkuppApi.new(**options)
      ApplicationController.markupp_api_factory = ->(**credentials) { fake_api.as(**credentials) }
      fake_api
    end
  end
end

module ActionDispatch
  class IntegrationTest
    # Faz login de verdade pelo formulário, contra a fake.
    def sign_in_as(email = FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD)
      post session_path, params: { session: { email:, password: } }
    end
  end
end
