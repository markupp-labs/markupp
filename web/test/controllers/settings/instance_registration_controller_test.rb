require "test_helper"

# Configurações da instância, seção Sign-up (ADR-0059): o administrador da instância abre e fecha o cadastro de cada
# método. O que quem hospeda fixou aparece sem botão, e com o cadastro desligado por quem hospeda a seção avisa que só o
# convite cria conta.
module Settings
  class InstanceRegistrationControllerTest < ActionDispatch::IntegrationTest
    ADMIN_ACCOUNTS = [ { email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, instance_admin: true } ].freeze
    GITHUB_LOCKED = [ { "id" => "github", "name" => "GitHub", "open" => true, "locked" => true } ].freeze

    def sign_in_admin(methods = FakeMarkuppInstanceRegistration::DEFAULT_METHODS)
      fake = use_fake_api(accounts: ADMIN_ACCOUNTS, instance_registration: methods)
      sign_in_as
      fake
    end

    test "an instance admin finds the sign-up section in the instance group" do
      sign_in_admin

      get settings_instance_registration_path

      assert_response :success
      assert_select "main h1", "Sign-up"
      assert_select "main nav a[href=?][aria-current=page]", settings_instance_registration_path, text: "Sign-up"
    end

    test "an account that does not administer the instance has no sign-up section and is sent away" do
      use_fake_api
      sign_in_as

      get settings_account_path
      assert_select "main nav a[href=?]", settings_instance_registration_path, count: 0

      get settings_instance_registration_path
      assert_redirected_to root_path
    end

    test "each method says whether it creates accounts, with a button to change it" do
      sign_in_admin([ { "id" => "local", "name" => "Email and password", "open" => true, "locked" => false },
                      { "id" => "github", "name" => "GitHub", "open" => false, "locked" => false } ])

      get settings_instance_registration_path

      assert_select "#registration_method_local", text: /Email and password.*Open/m do
        assert_select "button", "Close sign-up"
      end
      assert_select "#registration_method_github", text: /GitHub.*Closed/m do
        assert_select "button", "Open sign-up"
      end
    end

    test "closing a method saves it and says so" do
      fake = sign_in_admin

      patch settings_instance_registration_path, params: { registration: { method: "local", open: "false" } }

      assert_redirected_to settings_instance_registration_path
      follow_redirect!
      assert_select "[role=status]", "Saved. New accounts follow it from now on."
      assert_equal [ { id: "local", open: false } ], fake.instance_registration_updates
    end

    test "a method set by whoever hosts the instance has no button and says so, without naming settings" do
      sign_in_admin(GITHUB_LOCKED)

      get settings_instance_registration_path

      assert_select "#registration_method_github button", count: 0
      assert_select "#registration_method_github", text: /Set by whoever hosts this instance/
      assert_no_match(/MARKUPP_/, response.body)
    end

    test "a refused change goes back to the section with the reason" do
      sign_in_admin(GITHUB_LOCKED)

      patch settings_instance_registration_path, params: { registration: { method: "github", open: "false" } }

      assert_redirected_to settings_instance_registration_path
      assert_equal "Whoever hosts this instance set this method, so it cannot change here.", flash[:alert]
    end

    test "with sign-up turned off by whoever hosts the instance, the section says only invites create accounts" do
      sign_in_admin(GITHUB_LOCKED).close_registration!

      get settings_instance_registration_path

      assert_select "main p#registration_closed", "Whoever hosts this instance turned sign-up off: only invites create accounts."
    end
  end
end
