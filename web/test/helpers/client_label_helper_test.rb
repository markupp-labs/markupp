require "test_helper"

class ClientLabelHelperTest < ActionView::TestCase
  FIREFOX_LINUX = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0".freeze
  CHROME_MAC = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0 Safari/537.36".freeze
  EDGE_WINDOWS = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0 Safari/537.36 Edg/150.0".freeze
  SAFARI_IPHONE = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1".freeze
  CHROME_ANDROID = "Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0 Mobile Safari/537.36".freeze

  test "a browser shows as browser on system" do
    assert_equal [ "Firefox on Linux", "Chrome on macOS", "Edge on Windows", "Safari on iOS", "Chrome on Android" ],
                 [ FIREFOX_LINUX, CHROME_MAC, EDGE_WINDOWS, SAFARI_IPHONE, CHROME_ANDROID ].map { client_label(it) }
  end

  test "another client shows by its product name" do
    assert_equal [ "Claude-User", "markupp-cli", "curl" ], [ "Claude-User/1.0", "markupp-cli/1.2 (linux)", "curl/8.10" ].map { client_label(it) }
  end

  test "without user agent the client is unknown, in the locale" do
    assert_equal "Unknown client", client_label("")
    I18n.with_locale(:"pt-BR") { assert_equal "Cliente desconhecido", client_label(nil) }
  end

  test "the system label comes from the locale" do
    I18n.with_locale(:"pt-BR") { assert_equal "Firefox no Linux", client_label(FIREFOX_LINUX) }
  end
end
