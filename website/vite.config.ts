import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const repositoryName = process.env.GITHUB_REPOSITORY?.split("/")[1] ?? "IncidentGPT";
const customBase = process.env.VITE_BASE_PATH;

export default defineConfig({
  base: process.env.GITHUB_ACTIONS ? `/${repositoryName}/` : customBase ?? "/",
  plugins: [react()],
});
