require "test_helper"

class BusySpinnerHelperTest < ActionView::TestCase
  test "the spinner is an empty status region that the controller fills with the label" do
    span = Nokogiri::HTML5.fragment(busy_spinner).at_css("span")

    assert_equal "status", span["role"]
    assert_equal "busy-spinner", span["data-controller"]
    assert_equal "Loading", span["data-busy-spinner-label-value"]
    assert_nil span["data-busy-spinner-busy-when-value"]
    assert_empty span.text
  end

  test "the spinner waits for the selector it is given and takes extra classes" do
    span = Nokogiri::HTML5.fragment(busy_spinner(busy_when: "#search_results[aria-busy]", class: "absolute")).at_css("span")

    assert_equal "#search_results[aria-busy]", span["data-busy-spinner-busy-when-value"]
    assert_equal "busy-spinner absolute", span["class"]
  end

  test "the page spinner waits for a visit or a form that is not aimed at a frame, with the label hidden" do
    span = Nokogiri::HTML5.fragment(page_busy_spinner).at_css("span")

    assert_equal "html[aria-busy], form[aria-busy]:not([data-turbo-frame])", span["data-busy-spinner-busy-when-value"]
    assert_equal "busy-spinner busy-spinner-page busy-spinner-quiet", span["class"]
  end
end
