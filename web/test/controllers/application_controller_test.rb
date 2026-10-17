require "test_helper"

class ApplicationControllerTest < ActiveSupport::TestCase
  test "retry_minutes rounds a partial minute up" do
    assert_equal 2, ApplicationController.retry_minutes(90.seconds)
    assert_equal 1, ApplicationController.retry_minutes(30.seconds)
    assert_equal 3, ApplicationController.retry_minutes(3.minutes)
  end
end
