class SearchesController < ApplicationController
  def show
    @notes = Note.search(markupp_api, params[:q])
  end
end
