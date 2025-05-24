const grafanaConfig = require('@grafana/eslint-config/flat.js');
const stylistic = require('@stylistic/eslint-plugin-ts');
const typescriptParser = require('@typescript-eslint/parser');
const typescriptPlugin = require('@typescript-eslint/eslint-plugin');
const reactHooks = require('eslint-plugin-react-hooks');

module.exports = [
  {
    ignores: ['.config/**', 'dist/', 'node_modules/', 'coverage/', 'playwright-report/', 'test-results/'],
  },
  grafanaConfig,
  {
    // General JS/React rules, not specific to TS project parsing
    plugins: {
      '@stylistic': stylistic,
    },
    rules: {
      'react/prop-types': 'off',
    },
  },
  {
    files: ['src/**/*.{ts,tsx}', 'tests/**/*.{ts,tsx}', 'playwright/**/*.ts', '*.config.ts'],
    plugins: {
      '@typescript-eslint': typescriptPlugin,
    },
    languageOptions: {
      parser: typescriptParser,
      parserOptions: {
        project: './tsconfig.eslint.json',
      },
    },
    rules: {
      '@typescript-eslint/no-deprecated': 'warn',
    },
  },
  {
    files: ['*.config.js', '.config/**/*.js'],
    languageOptions: {
      globals: {
        module: 'readonly',
        require: 'readonly',
        process: 'readonly',
        __dirname: 'readonly',
      }
    },
    rules: {
    }
  },
];
