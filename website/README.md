# IncidentGPT Website

Static documentation site for IncidentGPT. The site is built with Vite, React, TypeScript, Mermaid and Lucide icons, and is designed for GitHub Pages.

## Local Development

```bash
npm ci
npm run dev
```

## Production Build

```bash
npm run build
npm run preview
```

The build output is written to `dist/`.

## GitHub Pages

The repository workflow in `.github/workflows/pages.yml` installs dependencies inside `website/`, runs checks, builds the site and deploys `website/dist` through GitHub Pages Actions.

`vite.config.ts` uses the repository name as the base path during GitHub Actions, so the expected project URL is:

```text
https://sersert.github.io/IncidentGPT/
```

Routing uses hash URLs such as `#/en/installation` and `#/ru/installation`, which avoids refresh-time 404s on GitHub Pages.

## Content Updates

- Pages live in `src/pages/`.
- Shared UI lives in `src/components/`.
- Navigation and configuration tables live in `src/data/`.
- Localization dictionaries live in `src/i18n/`.
- YAML generation logic lives in `src/lib/generator.ts`.

## Design System

The main CSS variables are in `src/styles/globals.css`:

- `--bg`, `--surface`, `--card`, `--border`
- `--text`, `--muted`
- `--ai`, `--k8s`, `--prom`
- `--success`, `--warning`, `--error`

The site supports dark, light and system themes. The selected theme and language are stored only in localStorage.
