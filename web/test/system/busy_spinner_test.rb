require "application_system_test_case"

# Indicador de carregamento em braille: só aparece se a espera passar de 300ms e some quando a resposta chega. A
# FakeSlowCalls segura a resposta da API até o teste soltar; o tempo até o indicador aparecer é medido daqui, antes da
# ação, então um indicador que aparecesse antes do limite seria visto antes dos 300ms. Como no VS Code, a espera de uma
# visita ou de um envio aparece no lugar da ação: na linha da árvore, no botão, no card, no item da paleta.
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

  test "a slow note opened from the tree spins next to its name, without a page indicator or the turbo bar" do
    visit note_path("1")
    @api.hold_calls!("GET /notes/2")

    assert_spinner_after_threshold(busy_row("ideas")) { within("nav") { click_on "ideas" } }
    assert_selector ".turbo-progress-bar", visible: :all
    assert_no_selector ".turbo-progress-bar"
    assert_no_selector ".busy-spinner-page", visible: :all

    @api.release_calls!
    assert_selector "nav a[aria-current=page]", text: "ideas"
    assert_no_spinner
    assert_no_selector "[aria-busy=true]"
  end

  test "a slow save spins inside the save button" do
    visit note_path("1")
    fill_in "Path", with: "/work/plan"
    @api.hold_calls!("PUT /notes/1")

    assert_spinner_after_threshold(".note-form button[aria-busy=true]") { click_on "Save" }
    assert_selector ".note-form button[aria-busy=true]", text: "Save"

    @api.release_calls!
    assert_text "Saved"
    assert_no_spinner
  end
  # Ctrl+S envia sem botão: a espera vai para o botão que o envio teria usado.
  test "a slow ctrl+s spins inside the save button too" do
    visit note_path("1")
    fill_in "Path", with: "/work/plan"
    @api.hold_calls!("PUT /notes/1")

    assert_spinner_after_threshold(".note-form button[aria-busy=true]") { find_field("Path").send_keys([ :control, "s" ]) }
    assert_selector ".note-form button[aria-busy=true]", text: "Save"

    @api.release_calls!
    assert_text "Saved"
    assert_no_spinner
  end

  test "a slow login spins inside the log in button" do
    click_on FakeMarkuppAccounts::DEFAULT_EMAIL
    click_on "Log out"
    fill_in "Email", with: FakeMarkuppAccounts::DEFAULT_EMAIL
    fill_in "Password", with: FakeMarkuppAccounts::DEFAULT_PASSWORD
    @api.hold_calls!("POST /auth/login")

    assert_spinner_after_threshold("button[aria-busy=true]") { click_on "Log in" }
    assert_selector "button[aria-busy=true]", text: "Log in"

    @api.release_calls!
    assert_selector "nav", text: FakeMarkuppAccounts::DEFAULT_EMAIL, wait: 10
    assert_no_spinner
  end

  test "a slow tenant card spins inside the card that was chosen" do
    visit tenants_path
    @api.hold_calls!("GET /notes")

    assert_spinner_after_threshold(".tenant-card-open[aria-busy=true]") { click_on "Acme research" }
    assert_selector ".tenant-card-open[aria-busy=true]", text: "Acme research"

    @api.release_calls!
    assert_field "Path", with: "/"
    assert_no_spinner
  end

  # A paleta fica aberta enquanto a página escolhida carrega, com a espera no item.
  test "a slow page from the palette spins in its item, with the palette still open" do
    visit note_path("1")
    @api.hold_calls!("GET /me/tenants")
    find("body").send_keys([ :control, :shift, "p" ])
    send_keys "tenants"

    assert_spinner_after_threshold("#command_palette [role=option][aria-busy=true]") { send_keys :enter }
    assert_selector "dialog#command_palette[open] [role=option][aria-busy=true]", text: "All tenants"

    @api.release_calls!
    assert_selector "h1", text: "Tenants"
    assert_no_spinner
  end

  # O menu fecha ao escolher; a espera vai para a linha sobre a qual ele agiu.
  test "a slow action from the tree menu spins on the row it acts on" do
    visit note_path("1")
    @api.hold_calls!("GET /notes")
    find("nav [role=treeitem]", text: "ideas").right_click

    assert_spinner_after_threshold(busy_row("ideas")) { click_on "New note here" }

    @api.release_calls!
    assert_field "Path", with: "/"
    assert_no_spinner
  end

  # Voltar mostra a página do cache do Turbo (as configurações têm cache; a nota não), que não pode ter guardado a
  # espera de quando saiu dela.
  test "going back shows the page without the spinner it had when it was left" do
    visit settings_path
    @api.hold_calls!("GET /notes/2")
    assert_spinner_after_threshold(busy_row("ideas")) { within("nav#sidebar") { click_on "ideas" } }
    @api.release_calls!
    assert_selector "nav a[aria-current=page]", text: "ideas"

    go_back
    assert_selector "main h1", text: "Account"
    assert_no_spinner
    assert_no_selector "[aria-busy=true]"
  end

  test "a slow search shows the spinner in the search box" do
    visit root_path
    @api.hold_calls!("GET /notes/search")
    find("body").send_keys([ :control, "k" ])

    assert_spinner_after_threshold("dialog .busy-spinner") { fill_in "Search notes", with: "Big" }

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

  # Como no VS Code: aos 300ms o rótulo do botão some sem ele mudar de largura e o glifo fica no meio dele, então nada
  # ao redor anda. O braille (sem reduced-motion) tem os pontos no alto da célula; o que se mede é o centro da tinta.
  test "a slow save hides the label, keeps the width of the button and centers the glyph on it" do
    emulate_motion_preference("no-preference")
    visit note_path("1")
    fill_in "Path", with: "/work/plan"
    width = find_button("Save").evaluate_script("this.getBoundingClientRect().width")
    @api.hold_calls!("PUT /notes/1")

    assert_spinner_after_threshold(".note-form button[aria-busy=true]") { click_on "Save" }
    save = find(".note-form button[aria-busy=true]")
    assert_in_delta width, save.evaluate_script("this.getBoundingClientRect().width"), 0.5
    assert_equal "rgba(0, 0, 0, 0)", save.evaluate_script("getComputedStyle(this).color")
    assert_glyph_centered_on save
  end

  test "the glyph spinning on a tree row is centered on the row" do
    emulate_motion_preference("no-preference")
    visit note_path("1")
    @api.hold_calls!("GET /notes/2")

    assert_spinner_after_threshold(busy_row("ideas")) { within("nav") { click_on "ideas" } }
    assert_glyph_centered_on find(busy_row("ideas")), axes: [ 1 ]
  end

  # O X nativo do campo de busca e o glifo, os dois no accent, pareciam um par de controles.
  test "the native clear button of the search box hides while the spinner shows" do
    visit root_path
    find("body").send_keys([ :control, "k" ])
    fill_in "Search notes", with: "Big"
    assert_selector "[role=option]", count: 1
    refute_equal "none", search_cancel_display
    @api.hold_calls!("GET /notes/search")

    assert_spinner_after_threshold("dialog .busy-spinner") { fill_in "Search notes", with: "Bi" }
    assert_equal "none", search_cancel_display

    @api.release_calls!
    assert_no_spinner
    refute_equal "none", search_cancel_display
  end

  test "the braille frames spin every 80ms, and reduced motion shows three still dots instead" do
    @api.hold_calls!("GET /notes/2")
    visit note_path("1")
    click_on "Read"
    assert_selector ".note-embed .busy-spinner-glyph"

    assert_equal [ [], "\"...\"" ], glyph_motion

    emulate_motion_preference("no-preference")
    assert_equal [ "braille-spin 800" ], glyph_motion.first
  end

  private
    def busy_row(name) = "nav [role=treeitem][aria-busy=true][data-path$='/#{name}']"

    def assert_spinner_after_threshold(selector)
      started = Process.clock_gettime(Process::CLOCK_MONOTONIC)
      yield
      assert_selector "#{selector} .busy-spinner-glyph"
      assert_operator Process.clock_gettime(Process::CLOCK_MONOTONIC) - started, :>=, SHOW_AFTER
    end

    def assert_no_spinner = assert_no_selector(".busy-spinner-glyph", visible: :all)

    # Centro da tinta do glifo contra o centro do container, até meio pixel. A tinta é a do "⠿" (os seis pontos que os
    # quadros usam) na fonte do glifo, medida pelo canvas a partir da linha de base, que um marcador vazio dá.
    def assert_glyph_centered_on(container, axes: [ 0, 1 ])
      box = container.evaluate_script("(({ left, top, width, height }) => [ left + width / 2, top + height / 2 ])(this.getBoundingClientRect())")
      ink = container.find(".busy-spinner-glyph").evaluate_script(<<~JS)
        (() => {
          const marker = Object.assign(document.createElement("span"), { style: "display: inline-block; width: 0; height: 0" })
          this.append(marker)
          const baseline = marker.getBoundingClientRect().bottom
          marker.remove()
          const context = document.createElement("canvas").getContext("2d")
          context.font = getComputedStyle(this).font
          const dots = context.measureText("⠿")
          const { left, width } = this.getBoundingClientRect()
          return [ left + width / 2, baseline + (dots.actualBoundingBoxDescent - dots.actualBoundingBoxAscent) / 2 ]
        })()
      JS
      axes.each { |axis| assert_in_delta box[axis], ink[axis], 0.5, "glyph ink #{ink} off the center #{box} of the container" }
    end

    def search_cancel_display
      find_field("Search notes").evaluate_script('getComputedStyle(this, "::-webkit-search-cancel-button").display')
    end

    # Do glifo: as animações do ::before (nome e duração) e o quadro que ele mostra agora.
    def glyph_motion
      find(".busy-spinner-glyph").evaluate_script(<<~JS)
        [ this.getAnimations({ subtree: true }).map((motion) => `${motion.animationName} ${motion.effect.getComputedTiming().duration}`),
          getComputedStyle(this, "::before").content ]
      JS
    end
end
