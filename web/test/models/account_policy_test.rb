require "test_helper"

class AccountPolicyTest < ActiveSupport::TestCase
  test "reads whether the server takes new accounts and by password" do
    fake_api = FakeMarkuppApi.new
    fake_api.close_password_registration!

    policy = AccountPolicy.fetch(fake_api)

    assert policy.registration_enabled
    assert_not policy.password_registration_open
  end

  test "an unreachable server keeps the form, and the server refuses if closed" do
    policy = AccountPolicy.fetch(FakeMarkuppApi.new(unreachable: true))

    assert policy.registration_enabled
    assert policy.password_registration_open
  end
end
