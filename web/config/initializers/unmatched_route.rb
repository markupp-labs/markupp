# Coringa do 404: o que não casou com rota nenhuma vai para errors#show. Fica num append porque as gems (turbo-rails)
# desenham as rotas delas depois do config/routes.rb, e a coringa ali as engoliria. O append roda a cada carga das rotas,
# depois de todos os arquivos; registrado aqui, no boot, ele não se repete quando o development recarrega as rotas.
# Em development ela também troca a página de rota inexistente do Rails pelo 404 do painel.
Rails.application.routes.append do
  match "*unmatched", to: "errors#show", via: :all, defaults: { code: "404" }, format: false
end
