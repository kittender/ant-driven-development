Feature: Win the game
  As a player
  I want to win by getting three of my marks in a row
  So that the game declares me the winner

  @capped
  Scenario Outline: X wins with three in a line
    Given the board is:
      | <r1> |
      | <r2> |
      | <r3> |
    And it is player "X"'s turn
    When player "X" plays at row <row>, column <col>
    Then player "X" should win
    And the game status should be "finished"

    Examples: Rows, columns and diagonals
      | r1    | r2    | r3    | row | col | line            |
      | X X . | O O . | . . . | 1   | 3   | top row         |
      | O O . | X X . | . . . | 2   | 3   | middle row      |
      | O O . | . . . | X X . | 3   | 3   | bottom row      |
      | X O . | X O . | . . . | 3   | 1   | left column     |
      | O X . | . X O | . . . | 3   | 2   | middle column   |
      | O . X | O . X | . . . | 3   | 3   | right column    |
      | X O . | O X . | . . . | 3   | 3   | main diagonal   |
      | O . X | O X . | . . . | 3   | 1   | anti-diagonal   |

  @capped
  Scenario Outline: O wins with three in a line
    Given the board is:
      | <r1> |
      | <r2> |
      | <r3> |
    And it is player "O"'s turn
    When player "O" plays at row <row>, column <col>
    Then player "O" should win
    And the game status should be "finished"

    Examples: A row, a column and a diagonal
      | r1    | r2    | r3    | row | col | line            |
      | O O . | X X . | X . . | 1   | 3   | top row         |
      | X O . | X O . | . . X | 3   | 2   | middle column   |
      | O X . | X O . | X . . | 3   | 3   | main diagonal   |

  @capped
  Scenario Outline: No moves allowed after a win
    Given player "X" has played at row 1, column 1
    And player "O" has played at row 2, column 1
    And player "X" has played at row 1, column 2
    And player "O" has played at row 2, column 2
    And player "X" has played at row 1, column 3
    When player "O" plays at row <row>, column <col>
    Then the move should be rejected with "Game is over"
    And the board should be unchanged

    Examples: Game over is reported before any other error
      | row | col | cell          |
      | 3   | 3   | empty         |
      | 1   | 1   | occupied      |
      | 0   | 1   | off the board |
