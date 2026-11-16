require "test_helper"

# Visão Links da barra lateral, como os painéis Backlinks e Outgoing links do Obsidian: num frame da nota aberta, as
# ligações para ela, as que ela faz, as menções não ligadas e as sugestões de ligação, cada seção com a contagem.
class NoteLinksControllerTest < ActionDispatch::IntegrationTest
  PLAN = { id: "plan", title: "plan", path: "work/plan.md" }.freeze
  ROADMAP = { id: "roadmap", title: "roadmap", path: "roadmap.md" }.freeze
  ALICE = { id: "alice", title: "alice", path: "people/alice.md" }.freeze
  DRAFT = { id: "draft", title: "draft", path: "draft.md" }.freeze
  IDEAS = { id: "ideas", title: "ideas", path: "ideas.md" }.freeze
  NOTES = [ PLAN, ROADMAP, ALICE, DRAFT, IDEAS ].map { { id: it[:id], path: it[:path], content: "" } }.freeze
  GRAPH = {
    backlinks: { "plan" => [ { source: ROADMAP, target: PLAN, cited_title: "plan", type: "explicit", predicate: nil, inferred_target: nil },
                             { source: ALICE, target: PLAN, cited_title: "plan", type: "typed", predicate: "owner", inferred_target: nil } ] },
    links: { "plan" => [ { source: PLAN, target: ALICE, cited_title: "alice", type: "typed", predicate: "author", inferred_target: nil },
                         { source: PLAN, target: nil, cited_title: "Ghost", type: "explicit", predicate: nil, inferred_target: nil },
                         { source: PLAN, target: ROADMAP, cited_title: "roadmap", type: "embed", predicate: nil, inferred_target: nil } ],
             "draft" => [ { source: DRAFT, target: nil, cited_title: "Road map", type: "explicit", predicate: nil, inferred_target: ROADMAP } ] },
    backmentions: { "plan" => [ { note: DRAFT, cited_title: "plan", prior: 0.5 } ] },
    suggestions: { "plan" => [ { note: IDEAS, score: 0.03, resource_allocation: 0.5, cosine: 0.2 } ] }
  }.freeze

  # Ligação por menção nos dois sentidos: draft cita o título de plan, e plan cita o de ideas.
  BY_MENTION = {
    backlinks: { "plan" => [ { source: DRAFT, target: PLAN, cited_title: "plan", type: "mentioned", predicate: nil, inferred_target: nil } ] },
    links: { "plan" => [ { source: PLAN, target: IDEAS, cited_title: "ideas", type: "mentioned", predicate: nil, inferred_target: nil },
                         { source: PLAN, target: ALICE, cited_title: "alice", type: "explicit", predicate: nil, inferred_target: nil } ] }
  }.freeze

  # Ligação por texto parecido: plan e ideas falam do mesmo jeito, sem título citado.
  BY_SIMILAR_TEXT = {
    links: { "plan" => [ { source: PLAN, target: IDEAS, cited_title: "", type: "related", predicate: nil, inferred_target: nil } ] }
  }.freeze

  setup do
    @api = use_fake_api(notes: NOTES, graph: GRAPH)
    sign_in_as
  end

  test "the view folds each section with its count, all open" do
    get note_links_path("plan")

    assert_select "turbo-frame#note_links[target=_top]"
    sections = css_select("details.links-section[open]").map { [ it["id"], it.at_css("summary .links-section-title").text, it.at_css("summary .links-section-count").text ] }
    assert_equal [ [ "links_backlinks", "Backlinks", "2" ], [ "links_outgoing", "Outgoing links", "3" ],
                   [ "links_mentions", "Unlinked mentions", "1" ], [ "links_suggestions", "Suggestions", "1" ] ], sections
  end

  test "backlinks open the note that links, with the property of a typed link" do
    get note_links_path("plan")

    assert_select "#links_backlinks a[href=?]", note_path("roadmap") do
      assert_select ".links-row-title", text: "roadmap"
      assert_select ".links-row-kind", count: 0
    end
    assert_select "#links_backlinks a[href=?]", note_path("alice") do
      assert_select ".links-row-folder", text: "people"
      assert_select ".links-row-kind", text: "owner"
    end
  end

  test "outgoing links show the kind and mark the unresolved with a dashed underline and the reason" do
    get note_links_path("plan")

    assert_select "#links_outgoing a[href=?] .links-row-kind", note_path("alice"), text: "author"
    assert_select "#links_outgoing a[href=?] .links-row-kind", note_path("roadmap"), text: "embed"
    assert_select "#links_outgoing .link-unresolved[title=?][tabindex='-1']", "No note named Ghost" do
      assert_select ".links-row-title", text: "Ghost"
      assert_select ".sr-only", text: "(No note named Ghost)"
    end
  end

  test "an unresolved link with an inferred note says maybe and opens that note" do
    get note_links_path("draft")

    assert_select "#links_outgoing a.link-unresolved[href=?][title=?]", note_path("roadmap"), "No note named Road map, maybe roadmap" do
      assert_select ".links-row-title", text: "Road map"
      assert_select ".links-row-maybe", text: "maybe roadmap"
      assert_select ".sr-only", text: "(No note named Road map)"
    end
  end

  test "an unlinked mention opens the note that cites the title, with the words it cites" do
    get note_links_path("plan")

    assert_select "#links_mentions a[href=?]", note_path("draft") do
      assert_select ".links-row-title", text: "draft"
      assert_select ".links-row-kind", text: "\"plan\""
    end
  end

  test "a suggestion opens the note and copies its wikilink by y or by the button" do
    get note_links_path("plan")

    assert_select "#links_suggestions a[href=?][aria-keyshortcuts=Y][data-wikilink=?]", note_path("ideas"), "[[ideas]]"
    assert_select "#links_suggestions button[aria-label=?][title=?][data-wikilink=?]", "Copy [[ideas]]", "Copy wikilink (y)", "[[ideas]]"
    assert_select "[role=status][data-note-links-target=status]"
  end

  test "suggestions list a full page, like the other sections" do
    many = Array.new(12) { |i| { note: { id: "s#{i}", title: "s#{i}", path: "s#{i}.md" }, score: 0.01, resource_allocation: 0, cosine: 0 } }
    use_fake_api(notes: NOTES, graph: { suggestions: { "plan" => many } })
    sign_in_as

    get note_links_path("plan")

    assert_select "#links_suggestions .links-section-count", text: "12"
  end

  test "an empty section says so and counts zero" do
    get note_links_path("roadmap")

    assert_equal %w[0 0 0 0], css_select(".links-section-count").map(&:text)
    assert_select "#links_backlinks .links-none", text: "No backlinks yet"
    assert_select "#links_outgoing .links-none", text: "No outgoing links"
    assert_select "#links_mentions .links-none", text: "No unlinked mentions"
    assert_select "#links_suggestions .links-none", text: "No suggestions"
  end

  test "a server that does not answer shows the failure in the frame" do
    @api.cut_connection!

    get note_links_path("plan")

    assert_response :bad_gateway
    assert_select "turbo-frame#note_links", text: /Can't reach the markupp server/
  end

  test "the page of a note loads its links in a lazy frame of the links view, and the new note says to open one" do
    cookies[:sidebar_view] = "links"

    get note_path("plan")

    assert_select "#sidebar_view_links turbo-frame#note_links[src=?][loading=lazy][target=_top][data-controller=note-links]", note_links_path("plan")
    assert_select "#sidebar_view_links", text: /Open a note to see its links/, count: 0

    get root_path

    assert_select "#sidebar_view_links turbo-frame#note_links", count: 0
    assert_select "#sidebar_view_links", text: /Open a note to see its links/
  end

  test "a link by mention says mention and, to the title and the screen reader, which title was cited" do
    use_fake_api(notes: NOTES, graph: BY_MENTION)
    sign_in_as

    get note_links_path("plan")

    assert_select "#links_backlinks a[href=?][title=?]", note_path("draft"), "draft.md, linked because the text cites plan" do
      assert_select ".links-row-kind", text: "mention"
      assert_select ".sr-only", text: "(linked because the text cites plan)"
    end
    assert_select "#links_outgoing a[href=?] .links-row-kind", note_path("ideas"), text: "mention"
    assert_select "#links_outgoing a[href=?] .links-row-kind", note_path("alice"), count: 0
  end

  test "a link by similar text says similar text and why, and is refused like any automatic link" do
    use_fake_api(notes: NOTES, graph: BY_SIMILAR_TEXT)
    sign_in_as

    get note_links_path("plan")

    assert_select "#links_outgoing a[href=?][title=?]", note_path("ideas"), "ideas.md, linked because the texts are similar" do
      assert_select ".links-row-kind", text: "similar text"
      assert_select ".sr-only", text: "(linked because the texts are similar)"
    end
    assert_select "#links_outgoing li:has(a[href='#{note_path('ideas')}']) button[data-links-refuse]"
  end

  test "a link by meaning says meaning and, to the title and the screen reader, that the notes cover the same subject" do
    use_fake_api(notes: NOTES, graph: { links: { "plan" => [ { source: PLAN, target: IDEAS, cited_title: "", type: "semantic", predicate: nil, inferred_target: nil } ] } })
    sign_in_as

    get note_links_path("plan")

    assert_select "#links_outgoing a[href=?][title=?]", note_path("ideas"), "ideas.md, linked because the notes cover the same subject" do
      assert_select ".links-row-kind", text: "meaning"
    end
    assert_select "#links_outgoing li form[action=?]", link_refusals_path
  end

  test "an automatic row refuses the link by its button and by r, with the panel confirm, and a written row does not" do
    use_fake_api(notes: NOTES, graph: BY_MENTION)
    sign_in_as

    get note_links_path("plan")

    assert_select "#links_backlinks li form[action=?][data-turbo-frame=note_links]", link_refusals_path do |form|
      assert_equal %w[draft plan], css_select(form.first, "input[type=hidden][name='note_ids[]']").map { it["value"] }
      assert_select "input[type=hidden][name=note_id][value=plan]"
      assert_select "button[data-links-refuse][aria-label=?][aria-keyshortcuts=R][title=?][data-turbo-confirm=?][data-confirm-button=?]",
                    "Refuse link with draft", "Refuse link (r)",
                    "Refuse the automatic link between draft and plan? It stays out until you undo it in the vault settings.", "Refuse link"
    end
    assert_select "#links_outgoing li:has(a[href='#{note_path('ideas')}']) button[data-links-refuse]"
    assert_select "#links_outgoing li:has(a[href='#{note_path('alice')}']) button[data-links-refuse]", count: 0
  end

  test "a viewer sees the link by mention without the refuse action" do
    use_fake_api(notes: NOTES, graph: BY_MENTION, tenants: FakeMarkuppTenants::VIEWER_TENANTS)
    sign_in_as

    get note_links_path("plan")

    assert_select ".links-row-kind", text: "mention", count: 2
    assert_select "button[data-links-refuse]", count: 0
  end
end
