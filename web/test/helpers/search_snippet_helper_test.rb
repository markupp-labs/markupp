require "test_helper"

class SearchSnippetHelperTest < ActionView::TestCase
  test "highlighted_snippet marks the matched terms" do
    assert_equal "the <mark>kubernetes</mark> cluster", highlighted_snippet("the \u0002kubernetes\u0003 cluster")
  end

  test "highlighted_snippet escapes the note text, inside and outside the marks" do
    snippet = "a <b>bold</b> \u0002<script>alert(1)</script>\u0003 & c"

    assert_equal "a &lt;b&gt;bold&lt;/b&gt; <mark>&lt;script&gt;alert(1)&lt;/script&gt;</mark> &amp; c", highlighted_snippet(snippet)
    assert_predicate highlighted_snippet(snippet), :html_safe?
  end

  test "highlighted_snippet drops a marker without its pair" do
    assert_equal "x <mark>y</mark> z", highlighted_snippet("x \u0002y\u0003 \u0002z")
    assert_equal "a b", highlighted_snippet("a\u0003 b")
  end

  test "highlighted_snippet is empty for no snippet" do
    assert_equal "", highlighted_snippet(nil)
  end
end
