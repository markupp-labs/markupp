require "test_helper"

class BrailleArtTest < ActiveSupport::TestCase
  DARK = 0
  LIGHT = 255

  test "a dark 2x4 block is the full braille cell" do
    assert_equal "⣿", BrailleArt.from_gray(Array.new(4) { [ DARK, DARK ] }, threshold: 128)
  end

  test "a light 2x4 block is the blank braille cell" do
    assert_equal "⠀", BrailleArt.from_gray(Array.new(4) { [ LIGHT, LIGHT ] }, threshold: 128)
  end

  test "each dark pixel lights its own dot" do
    pixels = [ [ DARK, LIGHT ], [ LIGHT, LIGHT ], [ LIGHT, LIGHT ], [ LIGHT, DARK ] ]

    assert_equal "⢁", BrailleArt.from_gray(pixels, threshold: 128)
  end

  test "cells fill lines left to right, four pixel rows per line" do
    pixels = Array.new(8) { |row| row < 4 ? [ DARK, DARK, LIGHT, LIGHT ] : [ LIGHT, LIGHT, DARK, DARK ] }

    assert_equal "⣿⠀\n⠀⣿", BrailleArt.from_gray(pixels, threshold: 128)
  end

  test "an image that does not split into 2x4 cells names its size" do
    error = assert_raises(ArgumentError) { BrailleArt.from_gray(Array.new(3) { [ DARK ] * 3 }, threshold: 128) }

    assert_match(/3x3.*multiple of 2x4/, error.message)
  end
end
