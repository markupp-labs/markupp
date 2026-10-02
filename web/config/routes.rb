Rails.application.routes.draw do
  get "up" => "rails/health#show", as: :rails_health_check

  resources :notes, only: [ :index, :show, :update ]
  resource :search, only: :show

  root "notes#index"
  resource :theme, only: :update
end
