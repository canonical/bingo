module.exports = {
  root: true,
  env: { browser: true, es2020: true },
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:react-hooks/recommended',
    'prettier',
  ],
  ignorePatterns: ['dist', '.eslintrc.cjs'],
  parser: '@typescript-eslint/parser',
  plugins: ['react-refresh'],
  rules: {
    'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
    'no-restricted-syntax': [
      'error',
      {
        selector: 'JSXAttribute[name.name="dangerouslySetInnerHTML"]',
        message: 'dangerouslySetInnerHTML is forbidden — use sanitizeContent() and JSX text nodes.',
      },
    ],
  },
}
