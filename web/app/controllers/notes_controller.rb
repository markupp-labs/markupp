class NotesController < ApplicationController
  rescue_from MarkuppApi::NotFound, with: :render_not_found

  before_action :require_tenant
  before_action :require_writer, only: :new
  before_action :set_tree, except: :destroy, unless: -> { request.format.json? }
  before_action :set_typed_path, only: %i[create update]

  # A tela padrão é uma nota nova em branco; viewer não cria, então só escolhe na árvore. Com path, o :e do modo Vim.
  def index
    return open_typed_path if params.key?(:path)

    @note = Note.blank(nil) if current_tenant.writer?
  end

  # Em json só caminho e conteúdo: o ![[nota]] da leitura busca daqui a nota embutida (editor/note_embeds.js),
  # com a sessão, o tenant e o papel de quem lê, sem a árvore.
  def show
    @note = Note.find(credential.api, params[:id])
    respond_to do |format|
      format.html
      format.json { render json: @note.attributes.slice("path", "content") }
    end
  end

  def new
    @note = Note.blank(params[:folder])
  end

  def create
    @note = Note.new(path: resolved_path, content: note_content_param)
    if @note.save(credential.api)
      redirect_to saved_note_destination, notice: t(".created")
    else
      render :new, status: path_error_status
    end
  end

  def update
    @note = Note.new(id: params[:id], updated_at: params[:last_modified_at])
    if @note.update(credential.api, { path: resolved_path, content: note_content_param }, force: params[:force] == "true")
      redirect_to saved_note_destination, notice: t(".saved")
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
    def note_params = params.expect(note: [ :path, :content ])

    # O caminho volta ao campo como foi digitado num erro; a origem (caminho lido, ou a pasta da nota nova) segue a mesma
    # para o próximo envio resolver o relativo a partir dela.
    def set_typed_path
      @typed_path = note_params[:path]
      @origin_path = params[:origin_path].to_s
    end

    def resolved_path = Note.new(path: @origin_path).resolve(@typed_path)

    # O caminho resolvido como o campo de caminho, a partir da nota aberta: a nota que já existe nele, ou uma nova com
    # ele, ainda não salva, como o :e do Vim num arquivo que não existe. A nova já vem com o "# nome", como o campo de
    # caminho põe na nota nova (note_form_controller.js), e o cabeçalho segue o caminho se ele mudar.
    def open_typed_path
      path = Note.new(path: params[:origin_path].to_s).resolve(params[:path])
      existing = @tree.notes_within.find { it.path == path }
      return redirect_to(note_path(existing)) if existing

      @note = Note.new(path:).tap { it.content = "# #{it.title}\n\n" } if current_tenant.writer?
    end

    # O :wq do modo Vim salva e sai para a nota padrão.
    def saved_note_destination = params[:quit] == "true" ? root_path : note_path(@note)

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
      return head :not_found if request.format.json?

      render :not_found, status: :not_found, formats: :html
    end
end
