// 📌 Contract, fixed by the human on 2026-10-28: the public API of the game rules, as types only.
// It follows ../../spec/README.md. Workers implement it in game.ts and must not change it;
// only a human changes a contract (autonomy setting change_contract = "human").

export type Player = 'X' | 'O';

/** One cell: a player's mark, or null when empty. */
export type Cell = Player | null;

export type Status = 'in progress' | 'finished';

/** The four rejection messages, in the order they are checked (spec/README.md). */
export type Rejection = 'Game is over' | 'Invalid position' | 'Not your turn' | 'Cell already taken';

export interface GameState {
  /** board[row - 1][column - 1]: rows and columns are 1 to 3 from the top left. */
  readonly board: readonly (readonly Cell[])[];
  readonly turn: Player;
  readonly status: Status;
  readonly winner: Player | null;
  readonly draw: boolean;
}

export type MoveResult =
  | { readonly ok: true; readonly state: GameState }
  | { readonly ok: false; readonly reason: Rejection; readonly state: GameState };

export interface Score {
  readonly X: number;
  readonly O: number;
  readonly draws: number;
}

/** A series of games: the current game and the running score. Restart keeps the score. */
export interface Match {
  readonly game: GameState;
  readonly score: Score;
}

export interface GameApi {
  /** An empty board, X to move, status "in progress". */
  newGame(): GameState;
  /** Sets a board directly from spec rows such as "X O .", with the given turn (X if omitted). */
  fromRows(rows: readonly string[], turn?: Player): GameState;
  /** Plays one move. A rejected move returns the state unchanged. Never throws. */
  play(state: GameState, player: Player, row: number, column: number): MoveResult;

  /** A new match: a new game and a score of 0, 0, 0. */
  newMatch(): Match;
  /** Plays a move in the match's game; when the move finishes the game, the score counts it once. */
  playInMatch(match: Match, player: Player, row: number, column: number): { readonly match: Match; readonly result: MoveResult };
  /** A new game in the same match; the score is unchanged. */
  restart(match: Match): Match;
}
