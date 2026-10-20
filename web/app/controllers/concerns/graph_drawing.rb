# Grafo em json pronto para o canvas (graph_view_controller.js), o mesmo para o cofre inteiro e para o local da nota.
module GraphDrawing
  extend ActiveSupport::Concern

  # Teto de nós que o canvas desenha; acima dele ficam os de maior PageRank.
  NODE_LIMIT = 2000
end
