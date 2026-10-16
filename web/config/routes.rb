Rails.application.routes.draw do
  get "up" => "rails/health#show", as: :rails_health_check

  resources :notes, only: [ :index, :show, :new, :create, :update, :destroy ]
  resource :search, only: :show

  root "notes#index"
  resource :theme, only: :update
  resource :session, only: [ :new, :create, :destroy ]
  resources :tenants, only: [ :index, :create ]
  resource :current_tenant, only: :update
end
