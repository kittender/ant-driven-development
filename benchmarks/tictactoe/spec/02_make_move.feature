Feature: Make a move
  As a player
  I want to place my mark on an empty cell
  So that I can work toward three in a row

  Background:
    Given a new game has started

  @capped
  Scenario: Player places a mark on an empty cell
    When player "X" plays at row 1, column 1
    Then the cell at row 1, column 1 should contain "X"
    And it should be player "O"'s turn

  @capped
  Scenario: Players alternate turns
    When player "X" plays at row 1, column 1
    And player "O" plays at row 2, column 2
    Then it should be player "X"'s turn

  @capped
  Scenario: Cannot play on an occupied cell
    Given player "X" has played at row 1, column 1
    When player "O" plays at row 1, column 1
    Then the move should be rejected with "Cell already taken"
    And the board should be unchanged
    And it should be player "O"'s turn

  @capped
  Scenario: Cannot play out of turn
    When player "O" plays at row 1, column 1
    Then the move should be rejected with "Not your turn"
    And the board should be unchanged
    And it should be player "X"'s turn

  @capped
  Scenario Outline: Cannot play outside the board
    When player "X" plays at row <row>, column <col>
    Then the move should be rejected with "Invalid position"
    And the board should be unchanged

    Examples:
      | row | col |
      | 0   | 1   |
      | 4   | 2   |
      | 2   | 0   |
      | 3   | 4   |

  @capped
  Scenario: Outside the board is reported before out of turn
    When player "O" plays at row 4, column 1
    Then the move should be rejected with "Invalid position"

  @capped
  Scenario: Out of turn is reported before an occupied cell
    Given player "X" has played at row 1, column 1
    When player "X" plays at row 1, column 1
    Then the move should be rejected with "Not your turn"
