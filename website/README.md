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

The site shares the operator UI’s `[i]` mark, surface palette, blue accent and semantic status colors. The interactive incident example contains illustrative data and makes no product API calls.

The site supports dark, light and system themes. The selected theme and language are stored only in localStorage.

## Ссылка на рабочий инстанс

`VITE_PRODUCT_URL=https://incident.example npm run build` включает кнопки входа и
перехода в продукт. Без переменной, при некорректном URL или URL с credentials
кнопки скрыты. Поддерживаются HTTP/HTTPS. В workflow Pages только основной
репозиторий имеет свой адрес по умолчанию; форки задают собственную repository
variable `INCIDENTGPT_PRODUCT_URL` либо оставляют её пустой.

Ссылки на разделы имеют вид `#/ru/configuration?s=generator`: их можно копировать,
открывать в новой вкладке и использовать после перезагрузки.

## Браузерные регрессии

Соберите `VITE_BASE_PATH=/IncidentGPT/ npm run build`, затем выполните
`npx playwright install chromium` и `npm run test:browser`. Проверяются прямые
ссылки на разделы, reload/new tab, Escape и восстановление фокуса мобильного
меню, RU/EN, все страницы, загрузка ресурсов, отсутствие горизонтального
переполнения, вкладки примера и темы. В CI собирается отдельный статический
кандидат с `/IncidentGPT/`, адресом продукта из настроек Pages и SHA-256
манифестом. Bundle, результаты и снимки экрана сохраняются в артефакт на 7 дней.
PR не публикует Pages. CI использует Chromium; локально можно задать
`PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` на существующий Chromium-браузер.
