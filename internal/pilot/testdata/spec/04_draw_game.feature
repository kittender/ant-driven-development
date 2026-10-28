Feature: Draw game
  As a player
  I want the game to end in a draw when the board is full with no winner
  So that we know nobody won

  @capped
  Scenario: Board fills up with no three in a row
    Given the board is:
      | X O X |
      | X O O |
      | O X . |
    And it is player "X"'s turn
    When player "X" plays at row 3, column 3
    Then the game should end in a draw
    And the game status should be "finished"

  @capped
  Scenario: Win on the last cell takes priority over a draw
    Given the board is:
      | X O X |
      | O X O |
      | O X . |
    And it is player "X"'s turn
    When player "X" plays at row 3, column 3
    Then player "X" should win
    And the game should not be a draw

  @capped
  Scenario: No moves allowed after a draw
    Given the board is:
      | X O X |
      | X O O |
      | O X X |
    When player "O" plays at row 1, column 1
    Then the move should be rejected with "Game is over"
    And the board should be unchanged
