require "test_helper"

# Cadastro público: email e senha criam a conta, que entra logada. A primeira conta da instalação vira owner; as outras
# só nascem com o registro ligado no servidor.
class RegistrationsControllerTest < ActionDispatch::IntegrationTest
  NEW_EMAIL = "caio@markupp.dev"

  test "new shows the account form in the public layout, back to the login before the title" do
    get new_registration_path

    assert_response :success
    assert_select "main a[href=?] ~ h1", new_session_path
    assert_select "main a[href=?]", new_session_path, text: "Back to log in"
    assert_select "h1", "Create an account"
    assert_select "form[action=?][method=post]", registration_path do
      assert_select "label[for=registration_email]", "Email"
      assert_select "input[type=email][name=?][required][autocomplete=email]", "registration[email]"
      assert_select "label[for=registration_password]", "Password"
      assert_select "input[type=password][name=?][required][minlength=15][autocomplete=new-password][aria-describedby=registration_password_hint]", "registration[password]"
      assert_select "#registration_password_hint", "Use at least 15 characters."
      assert_select "button[type=button][aria-controls=registration_password][aria-pressed=false][aria-label=?]", "Show password"
      assert_select "label[for=registration_password_confirmation]", "Confirm password"
      assert_select "input[type=password][name=?][required][autocomplete=new-password]", "registration[password_confirmation]"
      assert_select "button[type=button][aria-controls=registration_password_confirmation][aria-pressed=false][aria-label=?]", "Show password"
      assert_select "button", "Create account"
    end
    assert_select "nav#sidebar", count: 0
  end

  test "the login page points to the account creation" do
    get new_session_path

    assert_select "main", text: /No account yet\?/
    assert_select "a[href=?]", new_registration_path, text: "Create one."
  end

  test "create makes the account and signs in with it" do
    use_fake_api

    post registration_path, params: { registration: { email: NEW_EMAIL, password: "senha-nova-de-teste" } }

    assert_redirected_to root_path
    follow_redirect!
    follow_redirect! while response.redirect?
    assert_select "#account_menu", text: /#{NEW_EMAIL}/
  end

  test "an email that already has an account says so and points to the login" do
    use_fake_api

    post registration_path, params: { registration: { email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: "outra-senha-de-teste" } }

    assert_response :unprocessable_content
    assert_select "[role=alert]", text: /This email already has an account\./ do
      assert_select "a[href=?]", new_session_path, text: "Log in"
    end
    assert_select "input[name=?][value=?]", "registration[email]", FakeMarkuppAccounts::DEFAULT_EMAIL
  end

  test "a server that only creates accounts by invite says so" do
    use_fake_api.close_registration!

    post registration_path, params: { registration: { email: NEW_EMAIL, password: "senha-nova-de-teste" } }

    assert_response :unprocessable_content
    assert_select "[role=alert]", "This server only creates accounts by invite. Ask an admin for one."
  end

  test "a confirmation that differs from the password shows under it and creates no account" do
    use_fake_api

    post registration_path, params: { registration: { email: NEW_EMAIL, password: "senha-nova-de-teste", password_confirmation: "senha-nova-de-tesTe" } }

    assert_response :unprocessable_content
    assert_select "input[name='registration[password_confirmation]'][aria-invalid=true][aria-describedby=registration_password_confirmation_error]"
    assert_select "#registration_password_confirmation_error", "The passwords don't match."
    post registration_path, params: { registration: { email: NEW_EMAIL, password: "senha-nova-de-teste" } }
    assert_redirected_to root_path
  end

  test "a short password shows the minimum under the password field" do
    use_fake_api

    post registration_path, params: { registration: { email: NEW_EMAIL, password: "curta", password_confirmation: "curta" } }

    assert_response :unprocessable_content
    assert_select "input[name='registration[password]'][aria-describedby='registration_password_hint registration_password_error']"
    assert_select "#registration_password_error", "Use at least 15 characters."
  end

  test "a common password asks for another one" do
    use_fake_api

    post registration_path, params: { registration: { email: NEW_EMAIL, password: "qazwsxedcrfvtgb", password_confirmation: "qazwsxedcrfvtgb" } }

    assert_response :unprocessable_content
    assert_select "#registration_password_error", "This password is too common. Choose another one."
  end

  test "a password the API refuses shows under the password field" do
    use_fake_api

    post registration_path, params: { registration: { email: NEW_EMAIL, password: " " } }

    assert_response :unprocessable_content
    assert_select "input[name='registration[password]'][aria-invalid=true][aria-describedby='registration_password_hint registration_password_error']"
    assert_select "#registration_password_error", "This password is too long. Use a shorter one."
  end
  test "the login page hides the account creation on a server that only creates accounts by invite" do
    use_fake_api.close_registration!

    get new_session_path

    assert_select "a[href=?]", new_registration_path, count: 0
    assert_no_match(/No account yet\?/, response.body)
    assert_select "a[href=?]", new_invite_acceptance_path
  end

  test "the account creation page goes back to the login when nothing creates accounts" do
    use_fake_api.close_registration!

    get new_registration_path

    assert_redirected_to new_session_path
    assert_equal "This server only creates accounts by invite. Ask an admin for one.", flash[:alert]
  end

  test "a closed password registration leaves out the form and keeps the providers" do
    use_fake_api(login_providers: FakeMarkuppExternalLogin::DEFAULT_PROVIDERS).close_password_registration!

    get new_registration_path

    assert_response :success
    assert_select "form[action=?]", registration_path, count: 0
    assert_select "form[action=?] input[name=purpose][value=signup]", external_login_path("github")
  end

  test "the login page points to the account creation when only a provider creates accounts" do
    use_fake_api(login_providers: FakeMarkuppExternalLogin::DEFAULT_PROVIDERS).close_password_registration!

    get new_session_path

    assert_select "a[href=?]", new_registration_path, text: "Create one."
  end
end
