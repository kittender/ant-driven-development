// 📌 Test scaffold, fixed by the human on 2026-10-28 (like the contract): testers use it, nobody changes it.
//
// quickpickle types the world given to every step as `any`, which strict
// TypeScript cannot check. This file gives the steps a typed World and
// typed Given / When / Then. Import them from here, not from 'quickpickle':
//
//   import { Given, When, Then, type World } from './world';
//   When('I start a new game', (world) => { world.game = game.newGame(); });
//
// Each step still receives the world first, then the values of the
// expression ({string}, {int}) and, last, a DataTable when the step has one.
import { Given as QGiven, When as QWhen, Then as QThen } from 'quickpickle';
import type { GameState, Match, MoveResult } from '../../src/game/contract';

export { DataTable } from 'quickpickle';

/** What one scenario remembers between its steps. A new World starts with every scenario. */
export interface World {
  /** The game the scenario plays. Set it in a Given or When step before reading it. */
  game: GameState;
  /** The match, for the scenarios about restart and score. */
  match: Match;
  /** The result of the last move a step played. */
  result?: MoveResult;
  /** The game as it was before that move, to check "the board should be unchanged". */
  before?: GameState;
}

type StepFn<A extends unknown[]> = (world: World, ...args: A) => void | Promise<void>;
type Register = typeof QGiven;

function typed(register: Register) {
  return <A extends unknown[]>(expression: string | RegExp, step: StepFn<A>): void => {
    register(expression, step as unknown as Parameters<Register>[1]);
  };
}

export const Given = typed(QGiven);
export const When = typed(QWhen);
export const Then = typed(QThen);
