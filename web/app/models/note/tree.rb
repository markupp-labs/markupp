# Árvore de pastas montada a partir dos caminhos. A API não tem pasta, só caminho.
class Note::Tree
  Folder = Data.define(:name, :folders, :notes)

  def self.build(notes)
    root = new_branch
    notes.each { |note| branch_for(root, note.folder_names)[:notes] << note }
    to_folder("", root)
  end

  def self.new_branch = { folders: {}, notes: [] }

  def self.branch_for(root, folder_names)
    folder_names.reduce(root) { |branch, name| branch[:folders][name] ||= new_branch }
  end

  def self.to_folder(name, branch)
    folders = branch[:folders].sort.map { |child_name, child| to_folder(child_name, child) }
    Folder.new(name:, folders:, notes: branch[:notes].sort_by(&:name))
  end

  private_class_method :new_branch, :branch_for, :to_folder
end
