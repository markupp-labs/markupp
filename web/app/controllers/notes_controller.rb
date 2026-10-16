class NotesController < ApplicationController
  rescue_from MarkuppApi::NotFound, with: :render_not_found

  before_action :require_tenant
  before_action :require_writer, only: :new
  before_action :set_tree, except: :destroy

  def index
  end

  def show
    @note = Note.find(credential.api, params[:id])
  end

  def new
    @note = Note.untitled(params[:folder])
  end

  def create
    @note = Note.new(note_params)
    if @note.save(credential.api)
      redirect_to note_path(@note), notice: t(".created")
    else
      render :new, status: path_error_status
    end
  end

  def update
    @note = Note.new(id: params[:id], updated_at: params[:last_modified_at])
    if @note.update(credential.api, note_params, force: params[:force] == "true")
      redirect_to note_path(@note), notice: t(".saved")
    else
      render :show, status: path_error_status
    end
  rescue MarkuppApi::Conflict
    @conflict = true
    render :show, status: :conflict
  end

  def destroy
    Note.new(id: params[:id]).destroy(credential.api)
    redirect_to root_path, notice: t(".deleted")
  end

  private
    def note_params
      attributes = params.expect(note: [ :path, :content ])
      attributes.merge(content: Note.restore_line_breaks(attributes[:content], crlf: params[:crlf] == "true"))
    end

    # A API recusaria o POST; o formulário vazio nem abre.
    def require_writer
      redirect_to root_path, alert: t("markupp_api.errors.read_only") unless current_tenant.writer?
    end

    def set_tree
      @tree = Note::Tree.build(Note.all(credential.api))
    end

    def path_error_status = @note.errors.of_kind?(:path, :taken) ? :conflict : :unprocessable_content

    def render_not_found
      render :not_found, status: :not_found
    end
end
