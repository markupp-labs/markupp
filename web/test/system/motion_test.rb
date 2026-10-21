require "application_system_test_case"

# Movimento à moda do Hyprland (motion.css): entrada na easeOutQuint, saída curta e linear. Cada caso dispara a ação e
# lê as transições e animações que ela começou no mesmo script, sem esperar tempo nenhum.
class MotionTest < ApplicationSystemTestCase
  QUINT = "cubic-bezier(0.23, 1, 0.32, 1)".freeze
  POPIN_IN = { "opacity" => "410 #{QUINT}", "transform" => "410 #{QUINT}" }.freeze
  LAYER_IN = { "opacity" => "380 #{QUINT}", "transform" => "380 #{QUINT}" }.freeze
  LAYER_OUT = { "display" => "150 linear", "opacity" => "150 linear", "overlay" => "150 linear", "transform" => "150 linear" }.freeze
  AREA_MOVE = [ "::view-transition-new(root) area-in 410 #{QUINT} 1.5rem", "::view-transition-old(root) area-out 150 linear 1.5rem" ].freeze
  AREA_BACK = [ "::view-transition-new(root) area-in 410 #{QUINT} -1.5rem", "::view-transition-old(root) area-out 150 linear -1.5rem" ].freeze

  setup do
    emulate_motion_preference("no-preference")
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form
  end

  test "opening a note moves nothing" do
    visit note_path("1")

    assert_equal [ {}, {}, {} ], motions_after("", "main", ".note-editor-pane", "nav#sidebar")
  end

  # A coluna da barra no grid do body encolhe e cresce junto com o deslize da barra: o main acompanha pela borda
  # esquerda, sem transform próprio.
  test "the sidebar slides out fast and back in on the curve, with its column and the spine along" do
    visit root_path

    closing = motions_after("document.querySelector('.sidebar-spine').click()", "nav#sidebar", "body", ".sidebar-spine", "main")
    assert_equal [ { "display" => "150 linear", "transform" => "150 linear" },
                   { "grid-template-columns" => "150 linear" },
                   { "transform" => "150 linear" }, {} ], closing

    assert_no_selector "nav#sidebar"
    opening = motions_after("document.querySelector('.sidebar-spine').click()", "nav#sidebar", "body", ".sidebar-spine", "main")
    assert_equal [ { "transform" => "480 #{QUINT}" }, { "grid-template-columns" => "480 #{QUINT}" }, { "transform" => "480 #{QUINT}" }, {} ], opening
  end

  # A barra fechando para, e abrindo parte, com a borda dela sobre a linha da barra de atividades: sem duas linhas no
  # fim do recolher nem salto da borda no começo do expandir.
  test "the sidebar stops and starts with its edge on the line of the activity bar" do
    visit root_path

    closing = sidebar_edge_after_click(at: :end)
    assert_no_selector "nav#sidebar"
    opening = sidebar_edge_after_click(at: :start)

    assert_equal [ activity_bar_edge, activity_bar_edge ], [ closing, opening ].map { it.round(1) }
  end

  # O deslize leva a borda esquerda do main junto com a barra: o que encosta à direita no cabeçalho (o seletor de modo)
  # fica parado e o que encosta à esquerda (o caminho) anda sem saltos, quadro a quadro, do lugar onde estava.
  test "the note header holds still while the sidebar slides, only its left edge follows the sidebar" do
    visit note_path("1")

    [ :closing, :opening ].each do |way|
      start, frames = header_frames_while_sliding
      assert_operator frames.size, :>=, 2, way
      assert frames.all? { |switcher, _| (switcher - start.first).abs <= 1 }, "#{way}: the mode switcher moved: #{start} #{frames}"
      paths = [ start.last, *frames.map(&:last) ]
      steps = paths.each_cons(2).map { |before, after| after - before }
      assert(steps.all? { |step| way == :closing ? step <= 0.5 : step >= -0.5 }, "#{way}: the path went back: #{paths}")
      assert_operator steps.map(&:abs).max, :<, 200, "#{way}: the path jumped: #{paths}"
    end
  end

  # A entrada roda uma vez, na troca: depois dela, o mesmo conteúdo desenhado de novo (morph do Turbo, connect de
  # controller, re-render) ou a alça arrastada não anima nada.
  test "an entrance plays once and content already on screen never comes in again" do
    visit note_path("1")

    execute_script("document.querySelector('[data-note-mode-mode-param=split]').click()")
    wait_until_still ".note-preview"
    assert_equal({}, motions_of_redrawn(".note-preview"))
    assert_equal({}, motions_of_redrawn(".note-editor-pane"))

    2.times do
      execute_script("document.querySelector('.sidebar-spine').click()")
      wait_until_still "body"
    end
    wait_until_still "nav#sidebar"
    assert_equal({}, motions_of_redrawn("nav#sidebar"))
    assert_equal [ {} ], motions_after("document.documentElement.style.setProperty('--sidebar-width', '320px')", "body")
  end

  test "the pane that comes in pops in and the one that leaves is gone at once" do
    visit note_path("1")

    reading = motions_after("document.querySelector('[data-note-mode-mode-param=read]').click()", ".note-preview", ".note-editor-pane")
    assert_equal [ POPIN_IN, {} ], reading
    assert_equal "none", find(".note-editor-pane", visible: :all).evaluate_script("getComputedStyle(this).display")
  end

  test "the tenant switcher and the search come in as layers and leave fast" do
    visit root_path

    assert_equal [ LAYER_IN ], motions_after("document.querySelector('[popovertarget=tenant_switcher]').click()", "#tenant_switcher")
    wait_until_still "#tenant_switcher"
    assert_equal [ LAYER_OUT ], motions_after("document.querySelector('#tenant_switcher').hidePopover()", "#tenant_switcher")

    search = "dialog[data-search-target=dialog]"
    assert_equal [ LAYER_IN ], motions_after("document.querySelector('[data-action=\"search#open\"]').click()", search)
    wait_until_still search
    assert_equal [ LAYER_OUT ], motions_after("document.querySelector('#{search}').close()", search)
  end

  # Gaveta do celular: desliza da borda esquerda (translateX de -100%, sem popin nem fade) sobre o fundo escurecido e
  # sai rápida.
  test "the phone drawer slides in from the left edge over a darkened backdrop and leaves fast" do
    visit root_path
    page.current_window.resize_to(390, 844)
    wait_until_still "nav#sidebar"

    opening = evaluate_script(<<~JS)
      (() => {
        const drawer = document.querySelector("nav#sidebar")
        document.querySelector("[popovertarget=sidebar]").click()
        const [ slide ] = drawer.getAnimations()
        return [ drawer.getAnimations().map((motion) => motion.transitionProperty), slide.effect.getComputedTiming().duration,
                 slide.effect.getKeyframes()[0].transform ]
      })()
    JS
    assert_equal [ [ "transform" ], 380, "translate(-100%)" ], opening

    wait_until_still "nav#sidebar"
    backdrop = find("nav#sidebar").evaluate_script("getComputedStyle(this, '::backdrop').backgroundColor")
    assert_match(/\Argba?\(\d+, \d+, \d+, 0\.[1-9]/, backdrop)
    assert_equal [ { "display" => "150 linear", "overlay" => "150 linear", "transform" => "150 linear" } ],
                 motions_after("document.querySelector('nav#sidebar').hidePopover()", "nav#sidebar")
  ensure
    page.current_window.resize_to(1400, 900)
  end

  # Troca de área (notas, configurações, tenants, telas públicas) como a troca de workspace do Hyprland: a página que
  # sai some rápida e linear deslizando um pouco, a que entra vem na easeOutQuint. Voltar às notas desliza para o lado
  # oposto (--area-shift negativo). A barra lateral e a lombada, iguais dos dois lados, ficam paradas, também na
  # lista de tenants, que com tenant ativo tem a barra das notas.
  test "entering the settings and the tenants slides the area one way, going back to the notes the other, the sidebar holds still" do
    visit note_path("1")

    assert_equal AREA_MOVE, area_motions_of_visit(settings_account_path)
    assert_equal AREA_BACK, area_motions_of_visit(note_path("1"))
    assert_equal AREA_MOVE, area_motions_of_visit(tenants_path)
    assert_equal AREA_BACK, area_motions_of_visit(note_path("1"))
  end

  # Áreas em ordem fixa, como os workspaces 1, 2 e 3 do Hyprland: notas, tenants, configurações. Ir a uma de número
  # maior desliza para um lado e a uma menor para o outro, também pelo voltar e avançar do navegador.
  test "the areas go in a fixed order, notes, tenants, settings, and the slide follows it, through the history too" do
    visit note_path("1")

    assert_equal AREA_MOVE, area_motions_of_visit(tenants_path)
    assert_equal AREA_MOVE, area_motions_of_visit(settings_account_path)
    assert_equal AREA_BACK, area_motions_of_visit(tenants_path)
    assert_equal AREA_MOVE, area_motions_while("history.back()")
    assert_equal AREA_BACK, area_motions_while("history.back()")
    assert_equal AREA_MOVE, area_motions_while("history.forward()")
  end

  # As telas públicas ficam fora da ordem: entrar e sair delas desliza sempre do mesmo jeito, sem lado especial.
  test "logging in slides like any move from the public screens" do
    visit note_path("1")
    click_on FakeMarkuppAccounts::DEFAULT_EMAIL
    click_on "Log out"
    fill_in "Email", with: FakeMarkuppAccounts::DEFAULT_EMAIL
    fill_in "Password", with: FakeMarkuppAccounts::DEFAULT_PASSWORD

    motions = area_motions_while(%(document.querySelector("form[action='#{session_path}'] button").click()))
    assert_equal AREA_MOVE, motions & AREA_MOVE + AREA_BACK
  end

  test "going from a note to a note swaps at once" do
    visit note_path("1")

    assert_empty area_motions_of_visit(root_path)
    assert_empty area_motions_of_visit(note_path("1"))
  end

  test "logging out slides to the login, with the sidebar, its spine and the activity bar leaving along" do
    visit note_path("1")
    click_on FakeMarkuppAccounts::DEFAULT_EMAIL

    motions = area_motions_while(%(document.querySelector("form[action='#{session_path}'] button").click()))
    leaving = %w[activity-bar sidebar sidebar-spine].map { "::view-transition-old(#{it}) area-out 150 linear 1.5rem" }
    assert_equal (AREA_MOVE + leaving).sort, motions
  end

  test "reduced motion swaps the area at once" do
    emulate_motion_preference("reduce")
    visit note_path("1")

    assert_empty area_motions_of_visit(settings_account_path)
    assert_empty area_motions_of_visit(note_path("1"))
  end

  test "reduced motion swaps everything at once" do
    emulate_motion_preference("reduce")
    visit note_path("1")

    actions = [ "document.querySelector('[data-note-mode-mode-param=read]').click()",
                "document.querySelector('.sidebar-spine').click()",
                "document.querySelector('.sidebar-spine').click()",
                "document.querySelector('[popovertarget=tenant_switcher]').click()",
                "document.querySelector('#tenant_switcher').hidePopover()" ]
    actions.each do |action|
      assert_equal [ {}, {}, {}, {}, {} ], motions_after(action, "nav#sidebar", "main", ".sidebar-spine", ".note-preview", "#tenant_switcher"), action
    end
  end

  private
    # Borda direita da barra num ponto do deslize que o clique na lombada começa, com a transição parada ali.
    def sidebar_edge_after_click(at:)
      evaluate_script(<<~JS)
        (() => {
          document.querySelector('.sidebar-spine').click()
          const sidebar = document.querySelector('nav#sidebar')
          const slide = sidebar.getAnimations().find((animation) => animation.transitionProperty == 'transform')
          slide.pause()
          slide.currentTime = #{at == :end ? "slide.effect.getTiming().duration" : 0}
          const edge = sidebar.getBoundingClientRect().right
          slide.finish()
          return edge
        })()
      JS
    end

    def activity_bar_edge
      evaluate_script("document.querySelector('.activity-bar-side').getBoundingClientRect().right").round(1)
    end

    # A saída que interrompe uma entrada encurta (a transição revertida); a ação seguinte espera a anterior acabar.
    def wait_until_still(selector)
      assert_selector(selector, visible: :all) { |element| element.evaluate_script("this.getAnimations().length == 0") }
    end

    # Clica na lombada e mede, a cada quadro até o movimento acabar, o x do seletor de modo e do campo do caminho.
    def header_frames_while_sliding
      evaluate_async_script(<<~JS)
        const done = arguments[0]
        const header = [ document.querySelector(".note-form [role=group]"), document.querySelector("#note_path") ]
        const read = () => header.map((element) => element.getBoundingClientRect().left)
        const start = read(), frames = []
        document.querySelector(".sidebar-spine").click()
        const tick = () => {
          frames.push(read())
          document.getAnimations().length ? requestAnimationFrame(tick) : done([ start, frames ])
        }
        requestAnimationFrame(tick)
      JS
    end

    # Troca o elemento por uma cópia, como um morph ou re-render faria, e devolve as transições e animações da cópia.
    def motions_of_redrawn(selector)
      evaluate_script(<<~JS, selector)
        (() => {
          const element = document.querySelector(arguments[0])
          const copy = element.cloneNode(true)
          element.replaceWith(copy)
          return Object.fromEntries(copy.getAnimations().map((motion) => [ motion.transitionProperty || motion.animationName, true ]))
        })()
      JS
    end

    def area_motions_of_visit(path) = area_motions_while("Turbo.visit(#{path.to_json})")

    # Roda a ação e junta, quadro a quadro até o turbo:load (que vem depois do fim da view transition), as animações
    # dos pseudo-elementos da view transition: "pseudo nome duração curva deslize", em ordem. A visita inteira cabe no prazo do
    # script, que o Capybara tira do tempo de espera: 10s, como o login, para os Chromes em paralelo.
    def area_motions_while(action)
      using_wait_time(10) { collect_area_motions(action) }
    end

    def collect_area_motions(action)
      evaluate_async_script(<<~JS)
        const done = arguments[0], seen = new Set()
        let loaded = false
        addEventListener("turbo:load", () => (loaded = true), { once: true })
        const tick = () => {
          document.getAnimations().filter((motion) => motion.effect.pseudoElement?.startsWith("::view-transition")).forEach((motion) => {
            const pseudo = motion.effect.pseudoElement
            const style = getComputedStyle(document.documentElement, pseudo)
            const shift = style.getPropertyValue("--area-shift").trim()
            seen.add(`${pseudo} ${motion.animationName} ${motion.effect.getComputedTiming().duration} ${style.animationTimingFunction} ${shift}`)
          })
          loaded ? done([ ...seen ].sort()) : requestAnimationFrame(tick)
        }
        ;(() => { #{action} })()
        requestAnimationFrame(tick)
      JS
    end

    # Roda a ação e devolve, por seletor, as transições e animações que começaram: nome => "duração curva".
    def motions_after(action, *selectors)
      evaluate_script(<<~JS, selectors)
        (() => {
          const elements = arguments[0].map((selector) => document.querySelector(selector))
          ;(() => { #{action} })()
          return elements.map((element) => Object.fromEntries(element.getAnimations().map((motion) => {
            const timing = motion.effect.getComputedTiming()
            const easing = motion.transitionProperty ? timing.easing : getComputedStyle(element).animationTimingFunction
            return [ motion.transitionProperty || motion.animationName, `${timing.duration} ${easing}` ]
          })))
        })()
      JS
    end
end
