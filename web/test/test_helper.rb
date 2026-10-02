ENV["RAILS_ENV"] ||= "test"
require_relative "../config/environment"
require "rails/test_help"
Dir[File.expand_path("support/*.rb", __dir__)].each { |file| require file }

module ActiveSupport
  class TestCase
    # Run tests in parallel with specified workers
    parallelize(workers: :number_of_processors)

    teardown { Note.api = @original_api if @original_api }

    def use_fake_api(**options)
      @original_api = Note.api
      Note.api = FakeMarkuppApi.new(**options)
    end
  end
end
