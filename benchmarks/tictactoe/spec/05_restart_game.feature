Feature: Restart the game
  As a player
  I want to restart at any time
  So that we can play again without reloading

  @capped
  Scenario: Restart in the middle of a game
    Given player "X" has played at row 1, column 1
    And player "O" has played at row 2, column 2
    When I restart the game
    Then the board should have 9 empty cells
    And it should be player "X"'s turn
    And the game status should be "in progress"

  @capped
  Scenario: Restart after a game is finished
    Given player "X" has played at row 1, column 1
    And player "O" has played at row 2, column 1
    And player "X" has played at row 1, column 2
    And player "O" has played at row 2, column 2
    And player "X" has played at row 1, column 3
    When I restart the game
    Then the game status should be "in progress"
    And the board should have 9 empty cells
    And it should be player "X"'s turn
