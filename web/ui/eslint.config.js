import sveltePlugin from "eslint-plugin-svelte";
import tsEslint from "typescript-eslint";
import globals from "globals";

export default tsEslint.config(
  { ignores: ["node_modules/", "dist/", "public/"] },
  {
    extends: [...tsEslint.configs.recommended],
    files: ["**/*.ts"],
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "module",
    },
  },
  ...sveltePlugin.configs["flat/recommended"],
  {
    files: ["**/*.svelte"],
    languageOptions: {
      parserOptions: {
        parser: tsEslint.parser,
      },
    },
  },
  {
    rules: {
      "svelte/no-at-html-tags": "off",
    },
  },
);
