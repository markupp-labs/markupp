require "application_system_test_case"

# Movimento à moda do Hyprland (motion.css): entrada na easeOutQuint, saída curta e linear. Cada caso dispara a ação e
# lê as transições e animações que ela começou no mesmo script, sem esperar tempo nenhum.
class MotionTest < ApplicationSystemTestCase
  QUINT = "cubic-bezier(0.23, 1, 0.32, 1)".freeze
  POPIN_IN = { "opacity" => "410 #{QUINT}", "transform" => "410 #{QUINT}" }.freeze
  POPIN_OUT = { "display" => "150 linear", "opacity" => "150 linear", "transform" => "150 linear" }.freeze
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

  test "the sidebar slides out fast and back in on the curve, with the content and the spine along" do
    visit root_path

    closing = motions_after("document.querySelector('.sidebar-spine').click()", "nav#sidebar", "main", ".sidebar-spine")
    assert_equal [ { "display" => "150 linear", "transform" => "150 linear" },
                   { "workspace-slide-left" => "150 linear" },
                   { "transform" => "150 linear" } ], closing

    assert_no_selector "nav#sidebar"
    opening = motions_after("document.querySelector('.sidebar-spine').click()", "nav#sidebar", "main", ".sidebar-spine")
    assert_equal [ { "transform" => "480 #{QUINT}" }, { "workspace-slide-right" => "480 #{QUINT}" }, { "transform" => "480 #{QUINT}" } ], opening
  end

  test "the pane that comes in pops in and the one that leaves fades out fast" do
    visit note_path("1")

    reading = motions_after("document.querySelector('[data-note-mode-mode-param=read]').click()", ".note-preview", ".note-editor-pane")
    assert_equal [ POPIN_IN, POPIN_OUT ], reading
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

  test "the phone drawer comes in as a layer from the left edge" do
    visit root_path
    page.current_window.resize_to(390, 844)
    wait_until_still "nav#sidebar"

    assert_equal [ LAYER_IN ], motions_after("document.querySelector('[popovertarget=sidebar]').click()", "nav#sidebar")
    assert_match(/\A0px /, find("nav#sidebar").evaluate_script("getComputedStyle(this).transformOrigin"))
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
