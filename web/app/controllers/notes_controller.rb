class NotesController < ApplicationController
  rescue_from MarkuppApi::NotFound, with: :render_not_found

  before_action :require_tenant
  before_action :require_writer, only: :new
  before_action :set_tree, except: :destroy

  # A tela padrão é uma nota nova em branco; viewer não cria, então só escolhe na árvore.
  def index
    @note = Note.blank(nil) if current_tenant.writer?
  end

  def show
    @note = Note.find(credential.api, params[:id])
  end

  def new
    @note = Note.blank(params[:folder])
  end

  def create
    @note = Note.new(path: note_path_param(note_params[:title]), content: note_content_param)
    if @note.save(credential.api)
      redirect_to note_path(@note), notice: t(".created")
    else
      render :new, status: path_error_status
    end
  end

  def update
    @note = Note.new(id: params[:id], updated_at: params[:last_modified_at])
    if @note.update(credential.api, { path: note_path_param(note_params[:title]), content: note_content_param }, force: params[:force] == "true")
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
    def note_params = params.expect(note: [ :folder, :title, :content ])

    # Sem o campo markdown (nota nova) o nome ganha .md; nota existente sem .md continua sem.
    def note_path_param(title)
      Note.compose_path(folder: note_params[:folder], title:, markdown: params[:markdown] != "false")
    end

    def note_content_param = Note.restore_line_breaks(note_params[:content], crlf: params[:crlf] == "true")

    # A API recusaria o POST; o formulário vazio nem abre.
    def require_writer
      redirect_to root_path, alert: t("markupp_api.errors.read_only") unless current_tenant.writer?
    end

    def set_tree
      @tree = load_note_tree
    end

    def path_error_status = @note.errors.of_kind?(:path, :taken) ? :conflict : :unprocessable_content

    def render_not_found
      render :not_found, status: :not_found, formats: :html
    end
end
