Rails.application.routes.draw do
  get "up" => "rails/health#show", as: :rails_health_check

  resources :notes, only: [ :index, :show, :new, :create, :update, :destroy ]
  resource :search, only: :show

  root "notes#index"
  resource :theme, only: :update
  resource :editor_style, only: :update
  resource :session, only: [ :new, :create, :destroy ]
  resource :invite_acceptance, only: [ :new, :create ]
  resource :password_request, only: [ :new, :create ]
  resource :password_reset, only: [ :new, :create ]
  get "account" => redirect("/settings/account")
  resource :password_change, only: :create
  resource :email_verification, only: :create
  resource :email_confirmation, only: [ :new, :create ]
  resources :tenants, only: [ :index, :create ]
  resource :current_tenant, only: :update
  resources :my_invites, only: [ :index, :update, :destroy ]

  get "settings" => redirect("/settings/account"), as: :settings
  namespace :settings do
    resource :account, only: :show
    resource :appearance, only: :show
    resources :members, only: [ :index, :update, :destroy ]
    resources :invites, only: [ :index, :create, :destroy ]
  end
end
