import { defineConfig } from 'vitest/config';
import { quickpickle } from 'quickpickle';

// ./spec is this workspace's honey: seeded from ../spec, then owned by the colony here
// (ripeners propose changes; capping follows the autonomy settings).
// Each .feature file runs as a Vitest test file; step definitions live in tests/steps.
export default defineConfig({
  plugins: [quickpickle()],
  test: {
    include: ['spec/**/*.feature', 'tests/**/*.test.ts'],
    setupFiles: ['tests/steps/index.ts'],
    coverage: {
      provider: 'v8',
      include: ['src/**/*.ts'],
      exclude: ['src/main.ts', 'src/ui/**'],
      // json-summary and json are what the coverage instrument parses; text is for humans.
      reporter: ['text', 'json-summary', 'json'],
      reportsDirectory: 'coverage',
    },
  },
});
