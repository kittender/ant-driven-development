Feature: Start a new game
  As a player
  I want to start a new Tic-Tac-Toe game
  So that I can play against another player

  @capped
  Scenario: A new game has an empty board and X to move
    When I start a new game
    Then the board should have 9 empty cells
    And it should be player "X"'s turn
    And the game status should be "in progress"
