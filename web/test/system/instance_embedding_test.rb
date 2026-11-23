require "application_system_test_case"

# Embedding da instância pelo teclado: a paleta leva à seção, o foco começa no provedor escolhido, as setas trocam de
# provedor e Enter salva.
class InstanceEmbeddingTest < ApplicationSystemTestCase
  setup do
    use_fake_api(accounts: [ { email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, instance_admin: true } ])
    sign_in_through_form
  end

  test "the palette opens the embedding section and the keyboard picks the local model and saves" do
    visit settings_account_path
    find("body").send_keys([ :control, :shift, "p" ])
    find("#command_palette input[role=combobox]").send_keys("Instance embedding", :enter)

    assert_current_path settings_instance_embedding_path
    assert_equal "embedding_provider_none", page.evaluate_script("document.activeElement.id")
    page.active_element.send_keys(:down)
    page.active_element.send_keys(:enter)

    assert_selector "[role=status]", text: "Saved."
    assert_selector "input#embedding_provider_local[checked]", visible: :all
  end
end
