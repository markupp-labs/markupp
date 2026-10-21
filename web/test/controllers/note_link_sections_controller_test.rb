require "test_helper"

# Próxima página de uma seção da visão Links: o Show more no fim de uma seção cheia pede a página seguinte num frame
# próprio, que traz as linhas e, se ainda houver mais, outro Show more.
class NoteLinkSectionsControllerTest < ActionDispatch::IntegrationTest
  PLAN = { id: "plan", title: "plan", path: "plan.md" }.freeze

  setup do
    @api = use_fake_api(notes: [ { id: "plan", path: "plan.md", content: "" } ], graph: FakeCrowdedNoteLinks.new(PLAN, count: 230).graph)
    sign_in_as
  end

  test "a full section ends with show more, which loads the next page in its own frame" do
    get note_links_path("plan")

    NoteLinkSection::NAMES.each do |name|
      assert_select "#links_#{name} .links-item", count: 101
      assert_select "#links_#{name} li.links-more turbo-frame#links_#{name}_100[target=_top]" do
        assert_select "a[href=?][data-turbo-frame=links_#{name}_100][data-links-item]", "/notes/plan/links/#{name}?offset=100", text: "Show more"
        assert_select ".busy-spinner[data-busy-spinner-busy-when-value=?]", "#links_#{name}_100[aria-busy]"
      end
    end
  end

  test "the next page brings its rows and a show more while the section goes on" do
    get "/notes/plan/links/backlinks?offset=100"

    assert_select "turbo-frame#links_backlinks_100[target=_top]" do
      assert_select "a[href=?]", note_path("backlink-101")
      assert_select ".links-item", count: 101
      assert_select "li.links-more turbo-frame#links_backlinks_200 a[href=?]", "/notes/plan/links/backlinks?offset=200"
    end
  end

  test "the last page has no show more" do
    get "/notes/plan/links/suggestions?offset=200"

    assert_select "turbo-frame#links_suggestions_200" do
      assert_select ".links-item", count: 30
      assert_select "a[href=?][data-wikilink=?]", note_path("suggestion-230"), "[[suggestion 230]]"
    end
    assert_select ".links-more", count: 0
  end

  # A API trava o offset em 10000 e devolveria a mesma página para sempre: a página cheia nesse teto não oferece mais.
  test "a full page at the last offset the api serves has no show more" do
    use_fake_api(notes: [ { id: "plan", path: "plan.md", content: "" } ], graph: FakeCrowdedNoteLinks.new(PLAN, count: 10_100).graph)
    sign_in_as

    get "/notes/plan/links/suggestions?offset=10000"

    assert_select "turbo-frame#links_suggestions_10000 a[href=?]", note_path("suggestion-10100")
    assert_select ".links-more", count: 0
  end

  test "an offset past the last one the api serves is refused" do
    get "/notes/plan/links/backlinks?offset=10100"

    assert_response :bad_request
    assert_equal "Unknown offset \"10100\", expected 0 to 10000", response.body
  end

  test "a section that is not one of the view is not found" do
    get "/notes/plan/links/graph?offset=100"

    assert_response :not_found
  end

  test "a server that does not answer shows the failure in the frame of the page" do
    @api.cut_connection!

    get "/notes/plan/links/mentions?offset=100"

    assert_response :bad_gateway
    assert_select "turbo-frame#links_mentions_100", text: /Can't reach the markupp server/
  end
end
