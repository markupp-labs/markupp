class NotesController < ApplicationController
  rescue_from MarkuppApi::Error, with: :render_unreachable
  rescue_from MarkuppApi::NotFound, with: :render_not_found

  before_action :set_tree, except: :destroy

  def index
  end

  def show
    @note = Note.find(params[:id])
  end

  def new
    @note = Note.untitled(params[:folder])
  end

  def create
    @note = Note.new(note_params)
    if @note.save
      redirect_to note_path(@note), notice: t(".created")
    else
      render :new, status: path_error_status
    end
  end

  def update
    @note = Note.new(id: params[:id], updated_at: params[:last_modified_at])
    if @note.update(note_params, force: params[:force] == "true")
      redirect_to note_path(@note), notice: t(".saved")
    else
      render :show, status: path_error_status
    end
  rescue MarkuppApi::Conflict
    @conflict = true
    render :show, status: :conflict
  end

  def destroy
    Note.new(id: params[:id]).destroy
    redirect_to root_path, notice: t(".deleted")
  end

  private
    def note_params
      attributes = params.expect(note: [ :path, :content ])
      attributes.merge(content: Note.restore_line_breaks(attributes[:content], crlf: params[:crlf] == "true"))
    end

    def set_tree
      @tree = Note::Tree.build(Note.all)
    end

    def render_unreachable(error)
      Rails.logger.error({ event: "markupp_api_unreachable", message: error.message }.to_json)
      render :unreachable, status: :bad_gateway
    end

    def path_error_status = @note.errors.of_kind?(:path, :taken) ? :conflict : :unprocessable_content

    def render_not_found
      render :not_found, status: :not_found
    end
end
