import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import globals from 'globals';

export default tseslint.config(
  { ignores: ['dist', 'coverage', 'node_modules'] },
  js.configs.recommended,
  ...tseslint.configs.strict,
  {
    languageOptions: { globals: { ...globals.browser } },
    rules: {
      'no-empty': 'error',
      eqeqeq: 'error',
    },
  },
  {
    // Proof of the plot garden/game/domain.md: game rules never depend on the UI or the DOM.
    // The plot is only proposed, so its proof only warns: a warning is a fact for the colony, not a failed proof.
    // When the plot is grown (🍄), these two rules become 'error' and the Prover enforces them.
    files: ['src/game/**/*.ts'],
    rules: {
      'no-restricted-imports': ['warn', { patterns: [{ group: ['../ui/*', '../main'], message: 'src/game is pure: it must not import the UI (plot garden/game/domain.md).' }] }],
      'no-restricted-globals': ['warn', 'document', 'window', 'localStorage'],
    },
  },
);
