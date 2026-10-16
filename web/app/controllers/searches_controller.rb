class SearchesController < ApplicationController
  def show
    @notes = Note.search(credential.api, params[:q])
  end
end
