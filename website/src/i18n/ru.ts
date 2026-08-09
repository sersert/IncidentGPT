import type { Translation } from "./types";

export const ru = {
  meta: {
    title: "IncidentGPT — AI-разбор Kubernetes-инцидентов",
    description:
      "Open-source AIOps-помощник: обогащает Prometheus-алерты Kubernetes-контекстом и логами, связывает их в инциденты и отдаёт AI-разбор в Telegram и веб-интерфейс.",
  },
  navigation: {
    overview: "Обзор",
    architecture: "Архитектура",
    installation: "Установка",
    configuration: "Конфигурация",
    webUi: "Веб-интерфейс",
    logs: "Обогащение логами",
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
    recommended: "Что улучшить",
    currentBehavior: "Как сейчас",
    potentialRisk: "Риск",
  },
  hero: {
    title: "Разобраться в инциденте раньше, чем закончится шторм алертов",
    eyebrow: "Prometheus → Kubernetes → логи → Telegram и веб",
    description:
      "IncidentGPT обогащает алерты данными Prometheus, Kubernetes и выжимкой логов, связывает каскад событий в один инцидент и отдаёт инженерный черновик разбора в Telegram и веб-интерфейс.",
    installButton: "Начать установку",
    githubButton: "Открыть GitHub",
    architectureLink: "Посмотреть архитектуру",
    demoNote:
      "Пример демонстрационный. Реальный результат зависит от алертов, метрик, Kubernetes-контекста, логов и выбранной модели.",
    steps: ["Alertmanager", "Enricher", "Корреляция", "Sanitizer", "AI Worker", "Telegram и веб"],
  },
  home: {
    terminalTitle: "Логи Enricher",
    resultTitle: "Черновик разбора",
    featuresTitle: "Основные возможности",
    featuresSubtitle: "Контекст, корреляция и доставка в привычный on-call канал.",
    compareTitle: "Почему не просто отправить алерт в ChatGPT",
    regularApproach: "Обычный подход",
    incidentgptApproach: "IncidentGPT",
    compareConclusion:
      "Качество AI-разбора зависит от качества переданного контекста. IncidentGPT собирает этот контекст автоматически.",
    reliabilityTitle: "Сначала сырой алерт. Разбор — потом.",
    reliabilityBody:
      "Сначала IncidentGPT публикует исходный алерт. После этого система обращается к модели и публикует разбор. LLM не должна становиться точкой отказа системы оповещения.",
    quickStartTitle: "Быстрый старт",
    quickStartNote: "Перед установкой заполните Helm values и создайте Kubernetes Secrets.",
    humanNote:
      "IncidentGPT формирует инженерный черновик разбора. Финальное решение всегда принимает человек.",
    features: [
      {
        title: "Обогащение алертов",
        body: "Диапазонные запросы в Prometheus, объекты Kubernetes, состояние подов и нод, события и ссылки на runbook.",
      },
      {
        title: "Логи в промпте",
        body: "К алерту едет выжимка ошибок: повторы схлопнуты, счётчик ×N показывает масштаб, INFO отброшен. Поэтому модель называет ошибку, а не просит кого-то её посмотреть.",
      },
      {
        title: "Связывание в инцидент",
        body: "Каскад алертов одного namespace в ограниченном временном окне склеивается в группу. Одна группа — один инцидент с одним разбором.",
      },
      {
        title: "AI-разбор",
        body: "Модель получает лейблы, аннотации, метрики, логи и контекст Kubernetes, а не одно название алерта, и сообщает уверенность числом.",
      },
      {
        title: "Telegram и веб-интерфейс",
        body: "Сначала публикуются сырые алерты, затем сводка и разбор. Веб добавляет состав группы, отброшенный шум, контекст, журнал действий и прогнозы.",
      },
      {
        title: "Маскирование и деградация",
        body: "Sanitizer маскирует секреты и персональные данные до того, как что-либо покинет кластер. Без Redis алерты обрабатываются поодиночке, отказ LLM не скрывает исходный алерт.",
      },
    ],
    basicPromptItems: ["Название алерта", "Summary", "Severity"],
    contextItems: [
      "Лейблы алерта",
      "Аннотации",
      "Метрики Prometheus",
      "Тренды метрик",
      "Состояние подов",
      "Статус контейнеров",
      "Состояние нод",
      "События Kubernetes",
      "Деплойменты в нуле реплик",
      "Выжимка ошибок из логов",
      "Связанные алерты",
      "Ссылка на runbook",
      "Имя кластера",
    ],
  },
  pages: {
    architecture: {
      title: "Архитектура",
      description: "Компоненты, поток алерта, Redis-корреляция, маскирование данных и сценарии отказов.",
    },
    installation: {
      title: "Установка",
      description: "Пошаговый Kubernetes-мастер: зонтичный чарт, Telegram, OpenRouter, секреты и Alertmanager.",
    },
    configuration: {
      title: "Конфигурация",
      description: "Переменные окружения всех компонентов, PromQL-шаблоны, runbooks и генератор values.",
    },
    webUi: {
      title: "Веб-интерфейс",
      description: "Компоненты backend и ui, алерты против инцидентов, хранение, доступ и обратный канал разбора.",
    },
    logs: {
      title: "Обогащение логами",
      description: "Выжимка логов к алерту: что нужно от хранилища, настройки и что попадает в модель.",
    },
    examples: {
      title: "Примеры",
      description: "Webhook payload, тест каскада, PrometheusRule и production-настройки.",
    },
    troubleshooting: {
      title: "Решение проблем",
      description: "Где смотреть логи и как проверять Alertmanager, Redis, Telegram, OpenRouter, RBAC и веб-интерфейс.",
    },
    limitations: {
      title: "Ограничения",
      description: "Честные границы текущей корреляции, LLM, хранения, безопасности и масштабирования.",
    },
  },
  architecture: {
    headings: {
      systemMap: "Карта системы",
      alertFlow: "Путь алерта",
      sequence: "Последовательность запросов",
      correlation: "Корреляция",
      sanitization: "Граница маскирования",
      failures: "Сценарии отказов",
    },
    diagramTitles: { system: "Схема архитектуры", sequence: "Диаграмма последовательности" },
    alertFlowSteps: [
      "Prometheus вычисляет правило алерта.",
      "Alertmanager шлёт webhook на /alert.",
      "Enricher проверяет payload.",
      "Enricher запрашивает метрики вокруг startsAt.",
      "Enricher читает поды, ноды и события Kubernetes, включая деплойменты в нуле реплик.",
      "Enricher тянет схлопнутую выжимку ошибок из хранилища логов, если оно настроено.",
      "Сырой алерт сразу публикуется в Telegram.",
      "Алерт добавляется в группу в Redis.",
      "После CORR_SETTLE группа закрывается.",
      "Группа уходит в backend: он сохраняет инцидент и передаёт тело дальше.",
      "AI Worker обращается к модели через Sanitizer.",
      "Черновик разбора публикуется отдельным сообщением.",
      "ANALYSIS_CALLBACK_URL возвращает разбор в backend, и его видно в вебе.",
    ],
    flowLabels: {
      input: "Вход",
      action: "Действие",
      output: "Результат",
      dependency: "Зависимость",
      error: "Что может пойти не так",
    },
    flowSteps: [
      {
        title: "Алерт",
        input: "Webhook-payload от Alertmanager",
        action: "Проверка и нормализация алертов",
        output: "Кандидат EnrichedAlert",
        dependency: "Маршрут Alertmanager и DNS сервиса",
        error: "Битый JSON или пустой список алертов",
      },
      {
        title: "Обогащение",
        input: "Лейблы, аннотации, startsAt, generatorURL",
        action: "Запросы в Prometheus, чтение Kubernetes API и поиск по хранилищу логов",
        output: "PromSample, K8sContext, выжимка логов, заметки и подсказки",
        dependency: "Prometheus API, RBAC сервис-аккаунта, опционально хранилище логов",
        error: "Частичный контекст и предупреждения в логах enricher",
      },
      {
        title: "Корреляция",
        input: "Обогащённый алерт с namespace и fingerprint",
        action: "HSet в grp:{namespace}, ожидание CORR_SETTLE",
        output: "Одна группа для backend, а при выключенном backend — для ai-worker",
        dependency: "Redis",
        error: "Откат на отправку алерта поодиночке",
      },
      {
        title: "Сырое уведомление",
        input: "Обогащённый алерт",
        action: "POST /incident-raw и sendMessage в Telegram",
        output: "Сырой алерт виден в канале",
        dependency: "Права бота в канале",
        error: "Ответ telegram_error и запись в логах ai-worker",
      },
      {
        title: "Разбор моделью",
        input: "Контекст Prometheus, Kubernetes и логов",
        action: "Запрос chat completions через Sanitizer",
        output: "Вероятная первопричина, шаги исправления и уверенность",
        dependency: "OpenRouter или другой OpenAI-совместимый API",
        error: "Сырой алерт остаётся, разбор пропускается",
      },
      {
        title: "Итоговое сообщение",
        input: "Ответ модели",
        action: "Комментарий или отдельное сообщение в Telegram плюс обратный вызов в backend",
        output: "Инженерный черновик в канале и в вебе",
        dependency: "Telegram API, ANALYSIS_CALLBACK_URL для веба",
        error: "Ошибка в логах; у инженера остаётся сырой алерт",
      },
    ],
    correlationFacts: [
      { term: "Ключ корреляции", value: "grp:{namespace}" },
      { term: "Кто попадает в группу", value: "тот же namespace + пришёл в окно склейки" },
      { term: "Поле в Redis", value: "fingerprint алерта" },
      { term: "Время жизни", value: "CORR_WINDOW" },
      { term: "Задержка склейки", value: "CORR_SETTLE" },
    ],
    correlationCallout: {
      title: "Детерминированно, но не причинно",
      body: "Это не граф зависимостей. Два несвязанных инцидента в одном namespace могут склеиться, а каскад между namespace — потеряться. Корень внутри группы определяет уже модель.",
    },
    sanitizationParagraphs: [
      "Data Sanitizer — отдельный Go-сервис, который маскирует секреты и персональные данные до того, как что-либо дойдёт до внешней LLM, Telegram, логов или вебхуков: чувствительные ключи, токены Bearer/Basic/JWT, пароли в connection string, Kubernetes Secrets, при желании почты, телефоны и IP. Ответ модели проходит через него на обратном пути.",
      "Enricher и ai-worker ходят к нему с общим HMAC-ключом (SANITIZER_AUTH_SHARED_SECRET, одинаковый у всех трёх). В продакшене остаётся SANITIZER_FAIL_CLOSED=true: если Sanitizer отверг payload или недоступен, исходные данные дальше не уходят.",
    ],
    failureHeaders: { component: "Что недоступно", behavior: "Как ведёт себя система" },
    failures: [
      { component: "OpenRouter или LLM", behavior: "Сырой алерт уже опубликован; ai-worker пишет ошибку и пропускает разбор." },
      { component: "Redis", behavior: "Enricher предупреждает в логах и шлёт алерты поодиночке; backend держит инциденты в памяти до рестарта пода." },
      { component: "Prometheus API", behavior: "Предупреждения по запросам метрик пишутся в лог, обогащение продолжается на остальном контексте." },
      { component: "Kubernetes API", behavior: "Инициализация клиента или чтение может упасть; enricher продолжает без контекста K8s." },
      { component: "Хранилище логов", behavior: "Шаг обогащения молча пропускается, алерт едет без выжимки логов." },
      { component: "Sanitizer", behavior: "При SANITIZER_FAIL_CLOSED=true исходный payload не уходит ни в LLM, ни в Telegram." },
      { component: "Telegram", behavior: "ai-worker возвращает telegram_error на синхронных отправках и пишет ошибки асинхронных." },
      { component: "Enricher", behavior: "Alertmanager не может доставить алерт в IncidentGPT." },
      { component: "Backend", behavior: "Ломается групповой путь: инцидента в вебе нет, тело не уходит в ai-worker." },
      { component: "AI Worker", behavior: "Enricher или backend не может отправить подготовленный инцидент; ошибка пишется в лог." },
      { component: "Веб-интерфейс", behavior: "Теряется только интерфейс: алерты и разбор продолжают идти в Telegram." },
    ],
  },
  installation: {
    headings: {
      prerequisites: "Шаг 0. Что нужно заранее",
      telegram: "Шаг 1. Telegram",
      openrouter: "Шаг 2. OpenRouter",
      secrets: "Шаг 3. Секреты",
      helm: "Шаг 4. Установка зонтичного чарта",
      images: "Шаг 5. Образы",
      alertmanager: "Шаг 6. Alertmanager",
      verify: "Шаг 7. Открыть веб-интерфейс",
    },
    prerequisites: [
      "Кластер Kubernetes",
      "kubectl",
      "Helm 3",
      "kube-prometheus-stack",
      "Telegram-бот",
      "Канал в Telegram",
      "Группа обсуждения",
      "Ключ OpenRouter или другого OpenAI-совместимого API",
      "Ingress-контроллер для веб-интерфейса (опционально)",
      "Хранилище логов с API OpenSearch или Elasticsearch (опционально)",
    ],
    redisCallout: {
      title: "Redis входит в чарт",
      body: "С 0.2.0 Redis — сабчарт: один узел, диск 2 Gi, вытеснение выключено. Отдельно ставить нечего. Свой подключается через incidentgpt-redis.enabled: false и адрес в incidentgpt-backend.env.REDIS_ADDR и incidentgpt-enricher.env.redisAddr.",
    },
    telegramSteps: [
      "Создайте бота через @BotFather и сохраните токен как TELEGRAM_BOT_TOKEN.",
      "Создайте канал и привязанную к нему группу обсуждения.",
      "Добавьте бота администратором и туда, и туда.",
      "Напишите тестовое сообщение и найдите id канала и группы в getUpdates.",
    ],
    telegramPrivacyNote: "Privacy Mode отключать не обязательно: ai-worker не читает обычные сообщения группы.",
    telegramSecretsCallout: {
      title: "Секреты",
      body: "Никогда не публикуйте настоящий TELEGRAM_BOT_TOKEN в Git, Helm values или документации.",
    },
    openrouterNote:
      "Подойдёт любой провайдер с OpenAI-совместимым эндпоинтом chat completions: код шлёт на OPENROUTER_BASE_URL поля model, max_tokens и messages.",
    secretsIntro:
      "У Sanitizer и его клиентов общий HMAC-ключ, поэтому секрет создаётся один раз, а enricher и ai-worker на него ссылаются.",
    secretsNote:
      "Генератор values на странице конфигурации выдаёт файл, который ссылается на эти секреты через existingSecret и сам секретов не содержит.",
    helmIntro: "Все пять компонентов ставятся одним релизом: enricher, ai-worker, sanitizer, backend и ui, плюс Redis.",
    helmDisableNote:
      "Backend и ui выключаются через enabled: false у обоих. Тогда верните enricher прямой групповой путь: incidentgpt-enricher.env.groupBackendUrl на /incident-group ai-worker'а.",
    imagesIntro: "Образы публикуются в ghcr.io по каждому тегу v*, поэтому для знакомства собирать ничего не нужно.",
    imagesBuildNote: "Своя сборка по-прежнему поддерживается — укажите в values свой registry.",
    imagesArchNote: "Multi-arch можно собрать через linux/amd64,linux/arm64, но проверьте получившиеся образы в своём кластере.",
    alertmanagerNote:
      "Имя в route.receiver должно точно совпадать с receivers[].name. continue: true оставляет рабочими Slack, PagerDuty и остальные ресиверы.",
    verifyIntro:
      "Интерфейс публикуется своим Ingress, а basic-авторизация на nginx прикрывает и его, и /api/. В дефолтных values лежит admin/admin — для продакшена подложите свой Secret с ключом htpasswd.",
  },
  configuration: {
    headings: {
      upgrade: "Обновление с 0.1.0",
      enricherEnv: "Переменные enricher",
      aiWorkerEnv: "Переменные AI Worker",
      backendEnv: "Переменные backend",
      promql: "Настройка метрик Prometheus",
      runbooks: "Runbooks",
      generator: "Генератор Helm values",
    },
    tableHeaders: { variable: "Переменная", default: "Дефолт", example: "Пример", purpose: "Смысл" },
    upgradeCallout: {
      title: "Настройки хранилища логов переименованы",
      body: "ELASTICSEARCH_URL, ELASTICSEARCH_USER, ELASTICSEARCH_PASSWORD и ELASTICSEARCH_INDEX стали LOGS_STORE_*; в values elasticsearchUrl превратился в logsStoreUrl, а блок elasticsearch — в logsStore. Алиасов нет: старые ключи не читаются, и обогащение логами тихо отключается. Дефолты образов переехали на ghcr.io/sersert/incidentgpt-* и тег 0.2.0.",
    },
    backendIntro:
      "Backend и веб-интерфейс опциональны. Если выключить оба, путь доставки остаётся как в 0.1.0: enricher шлёт группы прямо в ai-worker, всё уходит в Telegram.",
    promqlIntro:
      "В шаблонах метрик работают плейсхолдеры Go-шаблонов: {{ .Namespace }}, {{ .Cluster }}, {{ .Service }}, {{ .Node }}, {{ .Instance }}, а с 0.2.0 ещё {{ .Pod }} и {{ .Container }} — запрос можно привязать к проблемному поду, а не ко всему namespace.",
    runbooksIntro:
      "У RUNBOOK_BASE_URL обрезается хвостовой слеш, и к нему добавляется имя алерта в нижнем регистре. KubePodCrashLooping превращается в https://runbooks.example.com/alerts/kubepodcrashlooping.",
    metricLabels: { unit: "Единица", trend: "Тренд", series: "Серии", cardinality: "Кардинальность" },
    env: {
      enricher: {
        PROMETHEUS_URL: { description: "Базовый URL HTTP API Prometheus. Enricher ходит в /api/v1/query_range." },
        PYTHON_BACKEND_URL: {
          description: "Фолбэк для одиночного алерта, когда корреляция через Redis выключена или недоступна.",
          note: "Имя в коде осталось от прошлой версии; Helm направляет переменную на ai-worker.",
        },
        GROUP_BACKEND_URL: {
          description: "Куда уходит собранная группа из Redis.",
          note: "С 0.2.0 зонтичный чарт направляет её в backend: тот сохраняет инцидент и передаёт тело в ai-worker. Если backend выключен, верните сюда /incident-group.",
        },
        RAW_BACKEND_URL: { description: "Куда сразу уходит сырой алерт, пока окно группы ещё открыто." },
        CLUSTER_NAME: { description: "Метка кластера в обогащённом payload и шаблонах метрик." },
        ENVIRONMENT: { description: "Метка окружения, добавляемая в контекст инцидента." },
        REDIS_ADDR: {
          description: "Адрес Redis для корреляции по namespace и окну.",
          note: "С 0.2.0 Redis входит в чарт, там это incidentgpt-redis:6379. Свой прописывается здесь и в incidentgpt-backend.env.REDIS_ADDR: компоненты ходят в Redis независимо.",
        },
        REDIS_PASSWORD: {
          description: "Пароль Redis. Чарт рендерит его обычным значением переменной.",
          note: "Что улучшить: поддержка secretKeyRef для продакшена.",
        },
        CORR_WINDOW: { description: "TTL ключа grp:{namespace} в Redis." },
        CORR_SETTLE: {
          description: "Сколько ждать следствий каскада перед склейкой группы.",
          note: "В Helm values стоит 40s — покрывает разброс срабатывания разных правил.",
        },
        RAW_DEDUP_TTL: { description: "TTL дедупликации сырых алертов по fingerprint через SetNX. 0s отключает ключ дедупликации." },
        PROM_RANGE_BEFORE: { description: "Окно метрик до startsAt алерта." },
        PROM_RANGE_AFTER: { description: "Окно метрик после startsAt алерта." },
        LOGS_STORE_URL: {
          description: "Поисковый API хранилища логов. Пусто — шаг обогащения логами молча пропускается.",
          note: "Переименована из ELASTICSEARCH_URL в 0.2.0. Старые имена не читаются, алиасов нет.",
        },
        LOGS_STORE_USER: { description: "Пользователь хранилища логов. Пароль приходит через LOGS_STORE_PASSWORD из logsStore.existingSecret." },
        LOGS_STORE_INDEX: { description: "Шаблон индекса, в котором ищутся строки логов по алерту." },
        LOGS_RANGE_BEFORE: { description: "Сколько логов смотреть до алерта." },
        LOGS_RANGE_AFTER: { description: "Сколько логов смотреть после алерта." },
        LOGS_MAX_LINES: { description: "Сколько схлопнутых групп ошибок класть в промпт." },
        LOGS_BASE_URL: {
          description: "База для ссылки на веб-интерфейс логов, которую добавляют в алерт.",
          note: "Не путать с LOGS_STORE_URL: здесь ссылка для инженера, а не адрес API, куда ходит enricher.",
        },
        SANITIZER_URL: { description: "Сервис Sanitizer, маскирующий секреты и персональные данные до выхода payload из кластера." },
        SANITIZER_AUTH_SHARED_SECRET: { description: "HMAC-ключ для запросов к Sanitizer. Должен совпадать с ключом в sanitizer и ai-worker." },
        RUNBOOK_BASE_URL: { description: "База для runbook_url вида base/alertname-в-нижнем-регистре." },
        ENRICH_CLUSTER_CONTEXT: { description: "Включает PromQL-шаблоны уровня кластера." },
        ENRICH_NODE_CONTEXT: { description: "Включает PromQL-шаблоны уровня ноды." },
        ENRICH_WORKLOAD_CONTEXT: { description: "Включает PromQL-шаблоны уровня workload." },
        ENRICH_EXTERNAL_CONTEXT: { description: "Включает PromQL-шаблоны внешних зависимостей." },
        ENRICH_K8S_CONTEXT: { description: "Включает обогащение через Kubernetes API внутри кластера." },
      },
      aiWorker: {
        LISTEN_ADDR: { description: "Адрес прослушивания для /healthz, /incident, /incident-raw и /incident-group." },
        OPENROUTER_API_KEY: { description: "Обязательна. Ключ OpenRouter или другого OpenAI-совместимого эндпоинта chat completions." },
        OPENROUTER_BASE_URL: { description: "Эндпоинт chat completions." },
        OPENROUTER_MODEL: { description: "Модель, которая уходит в payload запроса." },
        OPENROUTER_TIMEOUT_SECONDS: {
          description: "HTTP-таймаут запроса к модели.",
          note: "Старый ключ REQUEST_TIMEOUT_SECONDS кодом не читается.",
        },
        OPENROUTER_MAX_TOKENS: {
          description: "Максимум токенов в ответе модели.",
          note: "В Helm values стоит 2000.",
        },
        TELEGRAM_BOT_TOKEN: { description: "Обязательна. Токен бота от @BotFather." },
        TELEGRAM_CHANNEL_ID: { description: "Обязательна. Канал, куда идут сырые алерты, сводки и разбор." },
        TELEGRAM_THREAD_CHAT_ID: {
          description: "Id группы обсуждения. Ответы уходят к посту канала и появляются в связанной группе, если Telegram настроен именно так.",
        },
        TELEGRAM_PARSE_MODE: { description: "Режим разметки для sendMessage." },
        ANALYSIS_CALLBACK_URL: {
          description: "Куда вернуть готовый разбор, чтобы его показал веб-интерфейс.",
          note: "Появилась в 0.2.0. Без неё разбор уходит только в Telegram, а в вебе инцидент остаётся в состоянии ожидания.",
        },
        SANITIZER_URL: { description: "Sanitizer маскирует промпт перед вызовом модели и ответ модели перед отправкой в Telegram." },
        SANITIZER_AUTH_SHARED_SECRET: {
          description: "HMAC-ключ, общий с sanitizer и enricher. Значение должно совпадать у всех трёх компонентов.",
          note: "Сам Sanitizer по умолчанию держит SANITIZER_FAIL_CLOSED=true: если он отверг payload или недоступен, исходные данные не уходят ни в LLM, ни в Telegram.",
        },
      },
      backend: {
        LISTEN_ADDR: { description: "Адрес прослушивания API инцидентов, которым пользуется интерфейс." },
        AI_WORKER_URL: {
          description: "Куда передаётся тело группы после сохранения инцидента.",
          note: "Путь обязателен: без него форвард уходит в корень и получает 404. В Helm values указан /incident-group.",
        },
        FORWARD_TO_AI_WORKER: { description: "Сохраняет прежний путь в Telegram: backend передаёт тело группы дальше как есть." },
        REDIS_ADDR: {
          description: "Хранилище инцидентов. Без Redis сервис держит состояние в памяти, и любой рестарт пода стирает историю.",
          note: "В Helm values стоит incidentgpt-redis:6379.",
        },
        PROMETHEUS_URL: { description: "Источник метрик для дашборда и прогнозов по нодам." },
        LOGS_STORE_URL: { description: "Хранилище логов для вкладки «Логи» в инциденте. Пусто — вкладка не заполняется." },
        LOGS_STORE_INDEX: { description: "Шаблон индекса для вкладки «Логи»." },
      },
    },
    metrics: {
      workload_cpu_usage: {
        description: "Потребление CPU вокруг момента срабатывания алерта.",
        unit: "ядра CPU",
        series: "Одна или несколько серий на namespace/под после агрегации.",
        trend: "rising — вторая половина окна выше первой минимум на 15%.",
        risk: "Высокая кардинальность, если лейблы пода и контейнера не свернуть до попадания в контекст алерта.",
      },
      workload_memory_working_set: {
        description: "Working set памяти подов в namespace алерта.",
        unit: "байты",
        series: "Обычно одна серия на под после sum by pod.",
        trend: "falling — поздние точки ниже ранних минимум на 15%.",
        risk: "Память шумит при перезапусках подов; держите запрос в пределах namespace.",
      },
      workload_pod_restarts: {
        description: "Недавние перезапуски контейнеров в namespace.",
        unit: "число перезапусков",
        series: "Одна серия на под или контейнер, если не агрегировать.",
        trend: "spike — последняя точка больше среднего более чем вдвое.",
        risk: "В нагруженных namespace вывод по контейнерам разрастается; агрегируйте, где возможно.",
      },
    },
  },
  webUi: {
    headings: {
      components: "Компоненты",
      wiring: "Куда встраивается backend",
      alertsVsIncidents: "Алерт и инцидент — разные сущности",
      storage: "Хранение",
      access: "Доступ",
      screens: "Что показывает интерфейс",
      dataSources: "Источники данных",
    },
    componentsIntro:
      "Инциденты можно смотреть в браузере, а не только в Telegram. За это отвечают два компонента, оба входят в общий чарт:",
    components: [
      { name: "backend", body: "принимает группы алертов от enricher, хранит инциденты и передаёт тело дальше в ai-worker." },
      { name: "ui", body: "сам интерфейс: nginx со статикой и прокси на backend." },
    ],
    componentsDisable:
      "Оба отключаются через enabled: false — остаётся прежняя связка из 0.1.0 с выводом только в Telegram.",
    wiringIntro: "Backend стоит в разрыве группового пути, а сырой фид идёт напрямую в ai-worker:",
    wiringPassthrough:
      "Backend передаёт тело дальше как есть, на тот же эндпоинт, который enricher звал напрямую, — разбор в Telegram работает как прежде. Без ANALYSIS_CALLBACK_URL разбор уходит только в Telegram, а в интерфейсе инцидент остаётся в ожидании.",
    wiringNote: "В зонтичных values обе настройки уже сведены; они важны, если компоненты ставятся по отдельности.",
    alertsVsIncidents: [
      "Алерт — единица сырого потока от Alertmanager. Инцидент — группа связанных алертов с одним разбором. Один к одному они не соответствуют: у инцидента может быть два десятка алертов, и часть из них разбор пометит шумом. До 0.2.0 группа из 8 алертов превращалась в 8 инцидентов.",
      "В интерфейсе это два раздела, и видно, какой алерт в какую группу попал, а какой не собран вовсе.",
    ],
    storageIntro:
      "Инциденты живут в том же Redis, что enricher использует для корреляции. Он входит в чарт сабчартом incidentgpt-redis: один узел, без репликации и sentinel, диск 2 Gi.",
    storageAddress: "Адрес правится в двух местах: enricher и backend ходят в Redis независимо.",
    storageEviction:
      "Вытеснение намеренно выключено — maxmemory-policy остаётся noeviction. С любой политикой allkeys-* Redis под нагрузкой начал бы выбрасывать инциденты, а это хранилище, а не кэш.",
    storageCallout: {
      title: "Без Redis история не переживёт рестарт",
      body: "Сервис всё равно работает, но держит состояние в памяти: любой рестарт пода очищает историю. Для чего-то серьёзнее демо под архив нужна нормальная база.",
    },
    accessIntro:
      "Basic-авторизация на nginx прикрывает и интерфейс, и /api/. Пароль хэшируется bcrypt'ом в шаблоне и не попадает в кластер открытым текстом; для продакшена подложите свой Secret с ключом htpasswd.",
    accessProbe:
      "Проба готовности вынесена на /healthz без авторизации — иначе kubelet получал бы 401 и под никогда не становился ready. Имя вошедшего передаётся бэкенду заголовком X-Auth-User и попадает в журнал действий: видно, кто принял инцидент в работу и когда.",
    screens: [
      { name: "Инциденты", body: "список групп с числом склеенных алертов и краткой сутью разбора." },
      { name: "Инцидент", body: "состав группы, отброшенный шум с причиной, разбор целиком, логи из хранилища, контекст (namespace, под, нода, деплой) и журнал действий." },
      { name: "Алерты", body: "сырой поток и привязка к инцидентам." },
      { name: "Дашборд", body: "что горит, к чему идти первым, жив ли AI-пайплайн, шторм или норма." },
      { name: "Аналитика", body: "поток алертов, самые шумные источники, качество разбора." },
      { name: "Прогнозы", body: "ресурсы, которые упрутся в предел, если тренд сохранится (считает Prometheus через deriv()), и загрузка нод по отдельности: среднее по кластеру прячет забитую ноду." },
    ],
    dataSourcesIntro:
      "Настраиваются прямо в интерфейсе, в разделе «Интеграции». Разделены по назначению и совместимости API, а не по названию продукта.",
    dataSourceHeaders: { category: "Категория", type: "Тип", worksWith: "Кто подходит" },
    dataSources: [
      { category: "Метрики", type: "Prometheus API", worksWith: "Prometheus, VictoriaMetrics, Thanos, Mimir" },
      { category: "Метрики", type: "Zabbix API", worksWith: "Zabbix" },
      { category: "Логи", type: "OpenSearch / Elasticsearch API", worksWith: "OpenSearch, ELK" },
      { category: "Логи", type: "Loki API", worksWith: "Loki" },
    ],
    dataSourcesOutside:
      "Kubernetes и Telegram настраиваются вне интерфейса: первый работает через сервис-аккаунт пода, второй — через окружение ai-worker. В интерфейсе для них показано только состояние.",
  },
  logs: {
    headings: {
      why: "Почему логи меняют разбор",
      requirements: "Что нужно от хранилища",
      wiring: "Подключение хранилища",
      collector: "Пример сборщика: fluent-bit",
      prompt: "Что попадает в модель",
    },
    whyParagraphs: [
      "Кроме метрик и контекста Kubernetes enricher умеет прикладывать к алерту выжимку логов. Без логов модель пишет «проверь логи пода», с логами — называет конкретную ошибку. С 0.2.0 разбор не имеет права отправлять инженера за данными: собирать их — работа системы, а если их не хватает, модель пишет «Не хватает данных:» — это сигнал расширить сбор, а не задание человеку.",
      "Сбор логов в чарт не входит: в кластере обычно уже есть свой стек, и ставить второй незачем. Подойдёт любое хранилище с поисковым API OpenSearch или Elasticsearch.",
    ],
    requirementsIntro: "Enricher шлёт обычный _search и ждёт документы с полями:",
    fieldHeaders: { field: "Поле", content: "Что содержит" },
    fields: [
      { field: "@timestamp", content: "время строки" },
      { field: "log или message", content: "сам текст" },
      { field: "kubernetes.namespace_name", content: "namespace пода" },
      { field: "kubernetes.pod_name", content: "имя пода" },
    ],
    requirementsSource:
      "Такую структуру даёт fluent-bit с фильтром kubernetes — она же у filebeat и fluentd с аналогичным обогащением.",
    mappingCallout: {
      title: "Подвох с маппингом",
      body: "Строковые поля индексируются как text с подполем keyword, а точное совпадение term работает только по .keyword. Запрос без суффикса молча возвращает ноль документов — ошибки не будет, просто пустой результат. Enricher уже ходит по .keyword.",
    },
    wiringNote:
      "Без logsStoreUrl шаг обогащения молча пропускается, компонент остаётся рабочим и без логов. Тот же адрес нужен backend'у для вкладки «Логи» в инциденте.",
    baseUrlCallout: {
      title: "Не путать с logsBaseUrl",
      body: "logsBaseUrl задаёт базу для ссылки на веб-интерфейс логов, которую инженер открывает руками. logsStoreUrl — адрес API, куда ходит сам enricher.",
    },
    collectorIntro:
      "DaemonSet читает логи с нод. Фильтровать namespace лучше по пути к файлу — тогда лишние логи даже не читаются с диска:",
    collectorNote:
      "Для Elasticsearch вместо Name opensearch укажите Name es. Сборщику нужен hostPath к /var/log; политики вроде Kyverno disallow-host-path будут на это ругаться, и для DaemonSet'а сбора логов это неустранимо.",
    promptIntro: "Класть в промпт сотни строк нельзя — он раздувается, дорожает и топит важное в шуме. Поэтому enricher:",
    promptRules: [
      "отбрасывает INFO: причину ищут в ошибках;",
      "схлопывает повторы — из строки убираются времена, числа и хеши, всё остальное считается одной ошибкой;",
      "сортирует FATAL → ERROR → WARN, внутри уровня по частоте;",
      "берёт логи всего namespace, но под из алерта ставит первым: причина часто оказывается у соседа.",
    ],
    promptResult: "В результате пятьсот строк превращаются в пару десятков:",
    promptCounter: "Счётчик ×N — сам по себе сигнал: единичный сбой или шторм.",
  },
  examples: {
    headings: {
      direct: "Прямой тест enricher",
      cascade: "Тест каскада",
      prometheusRule: "Тест через PrometheusRule",
      production: "Production-настройки",
      telegramMock: "Как это выглядит в Telegram",
    },
    cascadeNote: "Все алерты в одном namespace и отправлены внутри CORR_SETTLE, поэтому должны склеиться в один ключ Redis.",
    prometheusRuleNote: "В примере стоит release: kps. Поменяйте на метку, которую селектит ваш Prometheus.",
    environments: ["Разработка", "Staging", "Production"],
    productionItems: [
      "Фиксируйте теги образов и версии чартов; тег этого релиза — 0.2.0.",
      "Держите ключи OpenRouter, Telegram и HMAC для Sanitizer в Kubernetes Secrets.",
      "Замените дефолтные admin/admin у веб-интерфейса на Secret с ключом htpasswd.",
      "Включите авторизацию и persistence у Redis; оставьте maxmemory-policy в noeviction — инциденты это хранилище, а не кэш.",
      "Задайте requests и limits; пробы во всех чартах уже есть.",
      "Оставьте SANITIZER_FAIL_CLOSED=true, чтобы недоступный Sanitizer не приводил к утечке сырых данных.",
      "Что улучшить: PodDisruptionBudget и поддержка secretKeyRef для Redis.",
      "Не пускайте секреты в логи и не включайте DEBUG_PROMPT и другую отладочную печать payload в продакшене.",
    ],
    mockCallout: {
      title: "Только макет",
      body: "Пример не обращается к Telegram API и не содержит настоящих chat id и токенов. Разбор по-русски, потому что системный промпт ai-worker написан по-русски; язык ответа меняется правкой промпта в исходниках.",
    },
  },
  troubleshooting: {
    cards: [
      {
        title: "Alertmanager не шлёт webhook",
        checks: ["имя ресивера не совпадает", "не тот DNS сервиса или namespace", "порт 9099 и путь /alert", "NetworkPolicy или пустые endpoints"],
      },
      {
        title: "PrometheusRule не срабатывает",
        checks: ["не та метка release", "селектор правил Prometheus не подходит", "PromQL не возвращает серий", "слишком длинный for", "не тот namespace"],
      },
      {
        title: "Enricher не получает метрики",
        checks: ["URL Prometheus", "endpoints сервиса", "NetworkPolicy", "синтаксис PromQL", "auth-прокси перед Prometheus"],
      },
      {
        title: "Нет контекста Kubernetes",
        checks: ["имя ServiceAccount", "ClusterRoleBinding", "права на pods/events/nodes", "ENRICH_K8S_CONTEXT"],
      },
      {
        title: "Redis: connection refused",
        checks: ["имя сервиса redis", "endpoints", "пароль", "NetworkPolicy", "адрес прописан и в enricher, и в backend"],
      },
      {
        title: "Алерты не склеиваются",
        checks: ["один namespace", "есть fingerprint или стабильные лейблы", "Redis работает", "отправлены внутри CORR_SETTLE", "ищите ALERT_BUFFERED и GROUP_SENT"],
      },
      {
        title: "Telegram возвращает ошибку",
        checks: ["токен бота", "id канала начинается с -100", "группа обсуждения привязана", "права администратора у бота", "ответ Telegram API в логах"],
      },
      {
        title: "Веб-интерфейс отвечает 502 на все запросы",
        checks: ["деплоймент backend поднят", "сервис называется incidentgpt-backend", "nginx в ui проксирует /api/ именно на это имя", "fullnameOverride не меняли"],
      },
      {
        title: "Инциденты остаются в ожидании разбора",
        checks: ["у ai-worker задан ANALYSIS_CALLBACK_URL", "groupBackendUrl у enricher указывает на ingest backend", "backend передаёт дальше с FORWARD_TO_AI_WORKER", "в AI_WORKER_URL есть путь /incident-group"],
      },
      {
        title: "В инциденте и промпте нет логов",
        checks: ["LOGS_STORE_URL пуст, и шаг молча пропускается", "шаблон индекса не совпадает", "в values остались старые имена ELASTICSEARCH_*", "доступы к хранилищу", "запросы term требуют подполя .keyword"],
      },
      {
        title: "OpenRouter возвращает 401",
        checks: ["не тот ключ", "Secret не примонтирован", "перевод строки в значении", "не тот эндпоинт", "переменной нет в поде"],
      },
    ],
  },
  limitations: {
    headings: {
      correlation: "Корреляция",
      llm: "LLM",
      storage: "Хранение",
      security: "Безопасность",
      scaling: "Масштабирование",
    },
    correlation: {
      behavior: "один namespace плюс временное окно. Ключ в Redis — grp:namespace, поле — fingerprint.",
      risk: "несвязанные инциденты в одном namespace могут склеиться, а каскад между namespace — потеряться.",
      recommended: "добавить топологию, данные каталога сервисов, корреляцию по деплоям, трейсы или граф причинности, когда потребуется более точная группировка.",
      items: [
        "Нет графа зависимостей сервисов.",
        "Нет семантической близости.",
        "Не учитывается inhibition алертов.",
        "Нет поиска по похожим инцидентам в истории.",
        "Нет модели владения сервисами кроме лейблов.",
      ],
    },
    llmItems: [
      "Модель может ошибаться и выдумывать.",
      "Качество ответа зависит от качества контекста.",
      "Команды нужно проверять перед выполнением.",
      "Разбор не должен быть единственным основанием для решений по инциденту.",
    ],
    llmCallout: { title: "Решает человек" },
    storage: {
      behavior: "инциденты и алерты живут в Redis — один узел, без репликации и sentinel, диск 2 Gi, вытеснение намеренно выключено.",
      risk: "без Redis backend держит состояние в памяти, и любой рестарт пода стирает историю; заполненный диск перестанет принимать новые инциденты, а не выбросит старые.",
      recommended: "нормальная база под архив и обучение на прошлых инцидентах.",
    },
    securityCallout: {
      title: "Чувствительные данные",
      body: "Sanitizer маскирует секреты, токены и персональные данные до выхода из кластера, но это фильтр с правилами, а не гарантия. Считайте модель внешней стороной: не расширяйте обогащение до полных дампов конфигурации и Kubernetes Secrets в расчёте на то, что маскирование поймает всё.",
    },
    security: {
      behavior: "веб-интерфейс закрыт basic-авторизацией на nginx, она прикрывает и интерфейс, и /api/. Имя вошедшего попадает в журнал действий через заголовок X-Auth-User.",
      risk: "одна общая учётка, ни ролей, ни SSO, ни пофамильного аудита кроме этого заголовка.",
      recommended: "поставить перед Ingress авторизующий прокси, если истории инцидентов нужен настоящий контроль доступа.",
    },
    scaling: [
      {
        title: "Таймеры в памяти процесса",
        behavior: "groupTimers живёт в процессе; несколько реплик enricher могут завести независимые склейки одного ключа Redis.",
        recommended: "Распределённая блокировка, очередь или consumer стрима.",
      },
      {
        title: "Конкуренция за группу в Redis",
        behavior: "HSet дедуплицирует по fingerprint, но склейка читает и удаляет ключ целиком.",
        recommended: "Сделать склейку идемпотентной и наблюдаемой.",
      },
      {
        title: "Повторная доставка webhook",
        behavior: "Дедупликация по fingerprint помогает, но полного контракта идемпотентности доставки нет.",
        recommended: "Вести учёт обработанных id алертов и групп.",
      },
      {
        title: "Лимиты частоты",
        behavior: "Telegram и LLM могут упереться в rate limit при большом шторме.",
        recommended: "Добавить очередь, backoff и управление частотой.",
      },
      {
        title: "Размер payload",
        behavior: "/incident-group читает до 8 MiB; метрики высокой кардинальности и выжимка логов дают большие промпты.",
        recommended: "Ограничить число серий, снизить LOGS_MAX_LINES и агрегировать до отправки.",
      },
      {
        title: "Стоимость",
        behavior: "Один вызов модели на группу всё равно дорог при шумных инцидентах.",
        recommended: "Подбирать модель и max_tokens под окружение.",
      },
    ],
  },
  generator: {
    title: "Генератор Helm values",
    description: "Секреты не запрашиваются. Создайте Kubernetes Secret отдельно и подключите его через existingSecret.",
    umbrella: "values-incidentgpt.yaml",
    alertmanager: "Alertmanager snippet",
    fields: {
      clusterName: "Имя кластера",
      prometheusUrl: "Prometheus URL",
      redisAddress: "Адрес Redis",
      corrWindow: "Окно корреляции",
      corrSettle: "Задержка склейки",
      openRouterModel: "Модель OpenRouter",
      telegramChannelId: "Id канала Telegram",
      telegramThreadChatId: "Id группы обсуждения",
      logsStoreUrl: "Адрес хранилища логов",
      uiHost: "Хост веб-интерфейса",
      registry: "Container registry",
      imageVersion: "Версия образов",
    },
  },
} as const satisfies Translation;
