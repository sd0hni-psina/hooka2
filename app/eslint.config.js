// https://docs.expo.dev/guides/using-eslint/
const { defineConfig } = require('eslint/config');
const expoConfig = require('eslint-config-expo/flat');

module.exports = defineConfig([
  expoConfig,
  {
    ignores: ['dist/*', '.expo/*', 'node_modules/*'],
  },
  {
    files: ['**/*.ts', '**/*.tsx'],
    rules: {
      '@typescript-eslint/no-explicit-any': 'error',
      'no-restricted-syntax': [
        'error',
        {
          selector: "CallExpression[callee.object.name='Date'][callee.property.name='now']",
          message: 'Use serverNow() from src/ws/clock.ts, tech.md section 7.',
        },
      ],
    },
  },
  {
    files: ['src/ws/clock.ts'],
    rules: { 'no-restricted-syntax': 'off' },
  },
  {
    files: ['jest.setup.js'],
    languageOptions: { globals: { jest: 'readonly', require: 'readonly' } },
  },
]);
