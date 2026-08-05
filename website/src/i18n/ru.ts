import type { Translation } from "./types";

export const ru = {
  meta: {
    title: "IncidentGPT — AI-разбор Kubernetes-инцидентов",
    description:
      "Open-source AIOps-помощник: обогащает Prometheus-алерты Kubernetes-контекстом, коррелирует инциденты и отправляет инженерный AI-разбор в Telegram.",
  },
  navigation: {
    overview: "Обзор",
    architecture: "Архитектура",
    installation: "Установка",
    configuration: "Конфигурация",
    examples: "Примеры",
    troubleshooting: "Решение проблем",
    limitations: "Ограничения",
  },
  common: {
    copy: "Копировать",
    copied: "Скопировано",
    download: "Скачать",
    reset: "Сбросить",
    top: "Наверх",
    github: "GitHub",
    search: "Поиск по документации",
    noResults: "Ничего не найдено",
    breadcrumbsHome: "Документация",
    language: "Язык",
    theme: "Тема",
    dark: "Тёмная",
    light: "Светлая",
    system: "Системная",
    recommended: "Рекомендованное улучшение",
    currentBehavior: "Текущее поведение",
    potentialRisk: "Риск",
  },
  hero: {
    title: "Understand incidents before the alert storm ends",
    description:
      "IncidentGPT обогащает алерты данными Prometheus и Kubernetes, связывает каскад событий в один инцидент и отправляет инженерный AI-разбор в Telegram.",
    installButton: "Начать установку",
    githubButton: "Открыть GitHub",
    architectureLink: "Посмотреть архитектуру",
    demoNote:
      "Пример демонстрационный. Реальный результат зависит от алертов, метрик, Kubernetes-контекста и выбранной модели.",
  },
  home: {
    terminalTitle: "Логи Enricher",
    resultTitle: "Черновик разбора",
    featuresTitle: "Основные возможности",
    compareTitle: "Почему не просто отправить алерт в ChatGPT",
    regularApproach: "Обычный подход",
    incidentgptApproach: "IncidentGPT",
    compareConclusion:
      "Качество AI-разбора зависит от качества переданного контекста. IncidentGPT собирает этот контекст автоматически.",
    reliabilityTitle: "Raw alert first. AI analysis second.",
    reliabilityBody:
      "Сначала IncidentGPT публикует исходный алерт. После этого система обращается к модели и публикует разбор. LLM не должна становиться точкой отказа системы оповещения.",
    quickStartTitle: "Быстрый старт",
    humanNote:
      "IncidentGPT формирует инженерный черновик разбора. Финальное решение всегда принимает человек.",
  },
  pages: {
    architecture: {
      title: "Архитектура",
      description: "Компоненты, поток алерта, Redis-корреляция и сценарии отказов.",
    },
    installation: {
      title: "Установка",
      description: "Пошаговый Kubernetes-мастер: Telegram, OpenRouter, Redis, Helm и Alertmanager.",
    },
    configuration: {
      title: "Конфигурация",
      description: "Переменные окружения, PromQL-шаблоны, runbooks и генератор values.",
    },
    examples: {
      title: "Примеры",
      description: "Webhook payload, тест каскада, PrometheusRule и production-настройки.",
    },
    troubleshooting: {
      title: "Решение проблем",
      description: "Где смотреть логи и как проверять Alertmanager, Redis, Telegram, OpenRouter и RBAC.",
    },
    limitations: {
      title: "Ограничения",
      description: "Честные границы текущей корреляции, LLM, безопасности и масштабирования.",
    },
  },
  generator: {
    title: "Генератор Helm values",
    description: "Секреты не запрашиваются. Создайте Kubernetes Secret отдельно и подключите его через existingSecret.",
    aiWorker: "values-ai-worker.yaml",
    enricher: "values-enricher.yaml",
    alertmanager: "Alertmanager snippet",
    fields: {
      clusterName: "Имя кластера",
      prometheusUrl: "Prometheus URL",
      redisAddress: "Redis address",
      corrWindow: "Correlation window",
      corrSettle: "Correlation settle",
      openRouterModel: "OpenRouter model",
      telegramChannelId: "Telegram channel ID",
      telegramThreadChatId: "Telegram discussion group ID",
      registry: "Container registry",
      imageVersion: "Image version",
    },
  },
} as const satisfies Translation;
