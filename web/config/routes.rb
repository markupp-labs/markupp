Rails.application.routes.draw do
  get "up" => "rails/health#show", as: :rails_health_check

  resources :notes, only: [ :index, :show, :new, :create, :update, :destroy ] do
    resource :links, only: :show, controller: "note_links"
    # Próxima página de uma seção da visão Links (o Show more).
    resources :link_sections, path: "links", param: :section, only: :show, controller: "note_link_sections",
                              constraints: { section: /backlinks|outgoing|mentions|suggestions/ }
    resource :graph, only: :show, controller: "note_graphs"
  end
  resource :graph, only: :show
  resource :search, only: :show
  resource :explorer, only: :show

  root "notes#index"
  resource :theme, only: :update
  resource :locale, only: :update
  resource :editor_style, only: :update
  resource :vim_keys, only: :update
  resource :session, only: [ :new, :create, :destroy ]
  resource :registration, only: [ :new, :create ]
  # Login externo (ADR-0054): a ida ao provedor e a volta dele, que o servidor registra como redirect_uri.
  post "auth/:provider" => "external_logins#create", as: :external_login
  get "auth/:provider/callback" => "external_logins#callback", as: :external_login_callback
  resource :invite_acceptance, only: [ :new, :create ]
  resource :password_request, only: [ :new, :create ]
  resource :password_reset, only: [ :new, :create ]
  get "account" => redirect("/settings/account")
  resource :password_change, only: :create
  resource :email_verification, only: :create
  resource :email_confirmation, only: [ :new, :create ]
  resources :tenants, only: [ :index, :new, :create, :destroy ]
  resource :current_tenant, only: :update
  resource :current_vault, only: :update
  resources :vaults, only: [ :create, :update, :destroy ]
  resources :publications, only: [ :new, :create ]
  resources :my_invites, only: [ :index, :update, :destroy ]
  resource :notifications, only: :show

  # Consentimento OAuth: o servidor manda o navegador para cá com o id do pedido.
  resources :oauth_authorizations, path: "oauth/authorize", only: :show do
    member do
      post :approve
      post :deny
    end
  end

  get "settings" => redirect("/settings/account"), as: :settings
  namespace :settings do
    resource :account, only: :show
    resources :authorizations, only: [ :index, :destroy ]
    resources :identities, only: :destroy
    resource :appearance, only: :show
    resource :editor, only: :show
    resource :agents, only: :show
    resource :general, only: :show, controller: "general"
    resources :members, only: [ :index, :update, :destroy ]
    resources :invites, only: [ :index, :create, :destroy ]
    resource :vault, only: :show, controller: "vault_general"
    resources :vault_members, path: "vault/members", only: [ :index, :create, :destroy ]
    resources :publications, only: [ :index, :update, :destroy ]
  end

  # Nota pública, lida sem conta (ADR-0048): o fim do endereço é o slug do título e o id público depois do último hífen.
  # Sem ponto no trecho: o .json do ![[nota]] é o formato.
  scope "p/:slug_and_id", constraints: { slug_and_id: /[A-Za-z0-9-]+/ } do
    get "/", to: "public_notes#show", as: :public_note
    get "search", to: "public_searches#show", as: :public_note_search
    get "graph", to: "public_graphs#show", as: :public_note_graph
  end

  # Páginas de erro: o ShowExceptions pede /<status> (config.exceptions_app). O resto que não casa com rota nenhuma é o
  # 404, na coringa de config/initializers/unmatched_route.rb, que entra depois das rotas das gems.
  match "/:code", to: "errors#show", via: :all, constraints: { code: /[45]\d\d/ }
end
