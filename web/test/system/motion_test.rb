require "application_system_test_case"

# Movimento à moda do Hyprland (motion.css): entrada na easeOutQuint, saída curta e linear. Cada caso dispara a ação e
# lê as transições e animações que ela começou no mesmo script, sem esperar tempo nenhum.
class MotionTest < ApplicationSystemTestCase
  QUINT = "cubic-bezier(0.23, 1, 0.32, 1)".freeze
  POPIN_IN = { "opacity" => "410 #{QUINT}", "transform" => "410 #{QUINT}" }.freeze
  LAYER_IN = { "opacity" => "380 #{QUINT}", "transform" => "380 #{QUINT}" }.freeze
  LAYER_OUT = { "display" => "150 linear", "opacity" => "150 linear", "overlay" => "150 linear", "transform" => "150 linear" }.freeze

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

    assert_equal [ LAYER_IN ], motions_after("document.querySelector('[data-action=\"search#open\"]').click()", "dialog")
    wait_until_still "dialog"
    assert_equal [ LAYER_OUT ], motions_after("document.querySelector('dialog').close()", "dialog")
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
