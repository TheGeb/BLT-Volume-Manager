import sveltePlugin from "eslint-plugin-svelte";
import tsEslint from "typescript-eslint";
import globals from "globals";

export default tsEslint.config(
  { ignores: ["node_modules/", "dist/", "public/"] },
  {
    extends: [
      ...tsEslint.configs.recommended,
      ...tsEslint.configs.strictTypeChecked,
      ...tsEslint.configs.stylisticTypeChecked,
    ],
    files: ["**/*.ts"],
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "module",
      parserOptions: {
        projectService: true,
      },
    },
  },
  ...sveltePlugin.configs["flat/recommended"],
  {
    files: ["**/*.svelte"],
    languageOptions: {
      parserOptions: {
        parser: tsEslint.parser,
        projectService: true,
        extraFileExtensions: ['.svelte'],
      },
    },
  },
  {
    rules: {
      "svelte/no-at-html-tags": "off",
    },
  },
);
