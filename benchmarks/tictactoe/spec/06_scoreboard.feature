Feature: Keep score across games
  As a player
  I want a running score of wins and draws
  So that we can play a best-of series

  Background:
    Given the score is X: 0, O: 0, Draws: 0

  @capped
  Scenario: X's score increases when X wins
    When player "X" wins a game
    Then the score should be X: 1, O: 0, Draws: 0

  @capped
  Scenario: O's score increases when O wins
    When player "O" wins a game
    Then the score should be X: 0, O: 1, Draws: 0

  @capped
  Scenario: Draw increments the draw count
    When a game ends in a draw
    Then the score should be X: 0, O: 0, Draws: 1

  @capped
  Scenario: Scores add up over several games
    When player "X" wins a game
    And a game ends in a draw
    And player "X" wins a game
    And player "O" wins a game
    Then the score should be X: 2, O: 1, Draws: 1

  @capped
  Scenario: Score survives a restart
    Given player "O" has won a game
    When I restart the game
    Then the score should be X: 0, O: 1, Draws: 0

  @capped
  Scenario: Restarting an unfinished game does not change the score
    Given player "X" has played at row 1, column 1
    When I restart the game
    Then the score should be X: 0, O: 0, Draws: 0
