require "test_helper"

class PanelExceptionsAppTest < ActiveSupport::TestCase
  STATIC_RESPONSE = [ 500, { "content-type" => "text/html" }, [ "static 500" ] ].freeze

  setup do
    @log = StringIO.new
    @static = FakeRecordingRackApp.new(STATIC_RESPONSE)
  end

  test "the panel answers the error page when it can" do
    panel_response = [ 404, {}, [ "panel 404" ] ]
    app = PanelExceptionsApp.new(panel: FakeRecordingRackApp.new(panel_response), static: @static, logger: Logger.new(@log))

    assert_equal panel_response, app.call("PATH_INFO" => "/404")
    assert_nil @static.received_env
  end

  # Se a própria página de erro quebra, o ShowExceptions só daria um texto puro: a estática de public/ entra no lugar.
  test "the static page answers when the panel error page itself breaks" do
    app = PanelExceptionsApp.new(panel: FakeCrashingRackApp.new, static: @static, logger: Logger.new(@log))

    assert_equal STATIC_RESPONSE, app.call("PATH_INFO" => "/500")
    assert_equal "/500", @static.received_env["PATH_INFO"]
    assert_match(/"event":"panel_error_page_failed".*fake error page broke on \/500/, @log.string)
  end
end
