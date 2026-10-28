import type { GameApi } from './contract';

// Implementation of the contract. Empty on purpose: the colony writes it,
// one capped scenario at a time, after the tests for that scenario exist and fail.

const notYet = (): never => {
  throw new Error('not implemented');
};

export const game: GameApi = {
  newGame: notYet,
  fromRows: notYet,
  play: notYet,
  newMatch: notYet,
  playInMatch: notYet,
  restart: notYet,
};
