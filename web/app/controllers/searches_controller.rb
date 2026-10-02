class SearchesController < ApplicationController
  def show
    @notes = Note.search(params[:q])
  end
end
