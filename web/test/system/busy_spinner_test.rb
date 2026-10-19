require "application_system_test_case"

# Indicador de carregamento em braille: só aparece se a espera passar de 300ms e some quando a resposta chega. A
# FakeSlowCalls segura a resposta da API até o teste soltar; o tempo até o indicador aparecer é medido daqui, antes da
# ação, então um indicador que aparecesse antes do limite seria visto antes dos 300ms.
class BusySpinnerTest < ApplicationSystemTestCase
  SHOW_AFTER = 0.3
  NOTES = [ { id: "1", path: "plan.md", content: "Before\n\n![[ideas]]\n" },
            { id: "2", path: "ideas.md", content: "# Big idea\n" } ].freeze

  setup do
    @api = use_fake_api(notes: NOTES, invites: [ { id: "invite-1", tenant_id: "tenant-2", email: FakeMarkuppAccounts::DEFAULT_EMAIL, role: "member" } ],
                        tenants: [ *FakeMarkuppTenants::DEFAULT_TENANTS, { id: "tenant-2", name: "Home lab", members: { "bia@markupp.dev" => "owner" } } ])
    sign_in_through_form
  end

  teardown { @api.release_calls! }

  test "a slow visit shows the spinner at the top instead of the turbo progress bar" do
    visit note_path("1")
    @api.hold_calls!("GET /notes/2")

    assert_spinner_after_threshold(".busy-spinner-page") { within("nav") { click_on "ideas.md" } }
    assert_selector ".turbo-progress-bar", visible: :all
    assert_no_selector ".turbo-progress-bar"
    assert_selector ".busy-spinner-page", text: "Loading", visible: :all

    @api.release_calls!
    assert_selector "nav a[aria-current=page]", text: "ideas.md"
    assert_no_spinner
  end

  test "a slow form submission shows the spinner at the top" do
    visit note_path("1")
    fill_in "Path", with: "/work/plan"
    @api.hold_calls!("PUT /notes/1")

    assert_spinner_after_threshold(".busy-spinner-page") { click_on "Save" }

    @api.release_calls!
    assert_text "Saved"
    assert_no_spinner
  end

  test "a slow search shows the spinner in the search box" do
    visit root_path
    @api.hold_calls!("GET /notes/search")
    find("body").send_keys([ :control, "k" ])

    assert_spinner_after_threshold("dialog .busy-spinner") { fill_in "Search notes", with: "Big" }
    assert_no_selector ".busy-spinner-page .busy-spinner-glyph"

    @api.release_calls!
    assert_selector "[role=option]", count: 1
    assert_no_spinner
  end

  test "slow invites show the spinner with its label in the tenant switcher" do
    visit root_path
    @api.hold_calls!("GET /me/invites")

    assert_spinner_after_threshold("#tenant_switcher .busy-spinner") { click_on "Switch tenant" }
    assert_selector "#tenant_switcher .busy-spinner", text: "Loading"

    @api.release_calls!
    within("#tenant_switcher") { assert_text "Join Home lab as member" }
    assert_no_spinner
  end

  test "a slow embed shows the spinner with its label inside the embed" do
    @api.hold_calls!("GET /notes/2")
    visit note_path("1")

    assert_spinner_after_threshold(".note-embed .busy-spinner") { click_on "Read" }
    assert_selector ".note-embed .busy-spinner", text: "Loading"
    # O rótulo vem do en.yml, como nos outros indicadores, e não de um texto fixo no JS.
    assert_selector ".note-embed .busy-spinner[data-busy-spinner-label-value='#{I18n.t("busy_spinner.label")}']"
    assert_selector ".note-embed-body[aria-busy=true]"

    @api.release_calls!
    assert_selector ".note-preview .note-embed h1", text: "Big idea"
    assert_no_selector ".note-embed-body[aria-busy]"
    assert_no_spinner
  end

  test "the braille frames spin every 80ms, and stand still on the first frame with reduced motion" do
    @api.hold_calls!("GET /notes/2")
    visit note_path("1")
    click_on "Read"
    assert_selector ".note-embed .busy-spinner-glyph"

    assert_equal [ [], "\"⠋\"" ], glyph_motion

    emulate_motion_preference("no-preference")
    assert_equal [ "braille-spin 800" ], glyph_motion.first
  end

  private
    def assert_spinner_after_threshold(selector)
      started = Process.clock_gettime(Process::CLOCK_MONOTONIC)
      yield
      assert_selector "#{selector} .busy-spinner-glyph"
      assert_operator Process.clock_gettime(Process::CLOCK_MONOTONIC) - started, :>=, SHOW_AFTER
    end

    def assert_no_spinner = assert_no_selector(".busy-spinner-glyph", visible: :all)

    # Do glifo: as animações do ::before (nome e duração) e o quadro que ele mostra agora.
    def glyph_motion
      find(".busy-spinner-glyph").evaluate_script(<<~JS)
        [ this.getAnimations({ subtree: true }).map((motion) => `${motion.animationName} ${motion.effect.getComputedTiming().duration}`),
          getComputedStyle(this, "::before").content ]
      JS
    end
end
