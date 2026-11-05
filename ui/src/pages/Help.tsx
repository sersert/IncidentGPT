import { Link } from 'react-router-dom'
import { ArrowRight, BookOpen, Search } from 'lucide-react'
import { Card } from '@/components/ui/Card.tsx'

const chapters = [
  { id: 'triage', number: '01', title: 'Найдите и примите инцидент' },
  { id: 'investigate', number: '02', title: 'Сверьте факты и гипотезы' },
  { id: 'resolve', number: '03', title: 'Подтвердите результат' },
  { id: 'handoff', number: '04', title: 'Передайте контекст' },
]

export function Help() {
  return (
    <div>
      <div className="mb-8 max-w-3xl">
        <h1 className="page-title">Руководство дежурного</h1>
        <p className="page-description">
          Как принять инцидент, проверить данные и передать результат следующей смене.
        </p>
        <Link
          to="/incidents?status=active"
          className="mt-4 inline-flex min-h-10 items-center gap-2 text-[13px] text-accent hover:underline"
        >
          Открыть активные инциденты
          <ArrowRight size={15} />
        </Link>
      </div>
      <div className="grid items-start gap-10 xl:grid-cols-[minmax(0,1fr)_220px]">
        <div className="guide-prose min-w-0 space-y-8 text-[14px] leading-[1.8] text-text-secondary">
          <section id="triage">
            <Card plain title="Найдите и примите инцидент">
              <p>
                На экране «Обзор» видны текущие инциденты и доступные измерения. Для полного списка перейдите
                в «Инциденты»: выберите статус и критичность, найдите название или текст анализа. Фильтры
                сохраняются в адресе — эту ссылку можно передать коллеге.
              </p>
              <ol className="list-decimal space-y-2 pl-5">
                <li>
                  Откройте инцидент и проверьте критичность, время регистрации и число связанных алертов.
                </li>
                <li>
                  Нажмите «Принять в работу» и «Назначить». Если сохранение завершилось ошибкой, повторите
                  действие после проверки доступа.
                </li>
                <li>На вкладке «Обзор и анализ» изучите состав группы, цепочку событий и доступные логи.</li>
              </ol>
              <p>
                Алерт — отдельный сигнал. Инцидент — группа сигналов с общим контекстом. Принятие в работу
                фиксирует ответственность, но не подтверждает причину.
              </p>
            </Card>
          </section>
          <section id="investigate">
            <Card plain title="Сверьте факты и гипотезы">
              <p>
                Сначала прочитайте анализ и откройте его доказательства: источник, время наблюдения и
                доступность. Гипотеза модели может быть ошибочной или относиться к предыдущему составу группы.
              </p>
              <ul className="list-disc pl-5">
                <li>
                  <strong className="text-text-primary">Расследование.</strong> Посмотрите недавние изменения
                  и покрытие источников. Обновите контекст, проверьте гипотезу или сравните до/после изменения
                  с известным временем.
                </li>
                <li>
                  <strong className="text-text-primary">Зависимости.</strong> Проверьте объяснение
                  группировки. Перенос или отделение алерта сохраняет историю; затем запросите новый анализ.
                </li>
                <li>
                  <strong className="text-text-primary">Решения и runbooks.</strong> Сравните прошлый случай с
                  текущими версиями и окружением. Текст runbook — материал для инженера, а не автоматически
                  выполняемая команда.
                </li>
              </ul>
              <p>
                Частичный ответ, недоступный источник и успешная проверка без результатов — разные состояния.
                Отсутствие данных не подтверждает, что сервис здоров.
              </p>
            </Card>
          </section>
          <section id="resolve">
            <Card plain title="Подтвердите результат">
              <p>
                После проверки восстановления запишите известное время и доказательства в блоке жизненного
                цикла на вкладке «Обзор и анализ». Сигнал resolved от Alertmanager сам по себе не подтверждает
                восстановление сервиса.
              </p>
              <p>
                На вкладке «Решения и runbooks» сохраните проверенную причину, восстановившее сервис действие,
                evidence проверки и ограничения по среде или версии. Если причина неизвестна, оставьте её
                неизвестной. Ошибочное подтверждение можно исправить или отозвать с сохранением истории.
              </p>
              <p>
                Закройте инцидент после завершения работы. «Закрыт» — административный статус, время
                восстановления фиксируется отдельно.
              </p>
            </Card>
          </section>
          <section id="handoff">
            <Card plain title="Передайте контекст">
              <p>
                Откройте «Отчёты и передача». Сгенерируйте черновик постмортема или передачи смены, проверьте
                ссылки на источники, дополните выводы и назначьте владельцев последующих действий.
              </p>
              <p>
                Сохраните правки и отметьте состояние ревью. При конфликте версий перечитайте актуальную
                запись перед повторным сохранением. Новая генерация создаёт отдельную версию и не заменяет
                отредактированный черновик. Markdown можно скачать для передачи коллеге.
              </p>
              <p>
                Переключение рабочих вкладок сохраняет введённые черновики в открытой странице. Перед уходом
                на другой экран, обновлением страницы или закрытием браузера сохраните изменения.
              </p>
            </Card>
          </section>
          <section id="statuses">
            <Card plain title="Что означают статусы">
              <dl className="grid gap-4 sm:grid-cols-2">
                {[
                  ['Активен', 'Новый или возвращённый в очередь инцидент.'],
                  ['В работе', 'Инцидент принят инженером.'],
                  ['Закрыт', 'Работа административно завершена.'],
                  ['Заглушён', 'Инцидент выведен из активной очереди; это не восстановление.'],
                ].map(([term, description]) => (
                  <div key={term} className="border-b border-border-soft pb-4">
                    <dt className="mb-1 font-medium text-text-primary">{term}</dt>
                    <dd>{description}</dd>
                  </div>
                ))}
              </dl>
            </Card>
          </section>
          <section id="questions">
            <Card plain title="Если что-то не получается">
              <div className="divide-y divide-border-soft">
                {[
                  [
                    'В очереди ничего нет',
                    'Сбросьте фильтры и проверьте поток алертов. Если данных нет и там, администратору нужно проверить доставку от Alertmanager и групповой путь в backend. Пустой новый стенд не содержит примеров.',
                  ],
                  [
                    'Отказ в доступе или данные недоступны',
                    'Повторите загрузку. Если ошибка остаётся, обратитесь к администратору: учётные данные интерфейса и backend должны совпадать. Ошибка не означает, что инцидентов нет.',
                  ],
                  [
                    'Проверки, история изменений или отчёты недоступны',
                    'Эти возможности включаются администратором отдельно и требуют настроенных источников. Одна форма интеграции не подтверждает, что сбор данных работает.',
                  ],
                  [
                    'Анализ долго не появляется',
                    'Проверьте состояние задания и число попыток в истории анализа. Не повторяйте запрос без необходимости. Администратору нужно проверить доступность worker, sanitizer и модельного провайдера.',
                  ],
                ].map(([question, answer]) => (
                  <details key={question} className="py-4">
                    <summary className="cursor-pointer font-medium text-text-primary">{question}</summary>
                    <p className="mt-3 leading-relaxed">{answer}</p>
                  </details>
                ))}
              </div>
            </Card>
          </section>
        </div>
        <aside aria-label="В этой инструкции" className="space-y-5 xl:sticky xl:top-24">
          <Card plain title="В этой инструкции">
            <nav className="space-y-1">
              {chapters.map((chapter) => (
                <a
                  key={chapter.id}
                  href={`#${chapter.id}`}
                  className="flex items-start gap-3 rounded-lg py-3 text-xs text-text-secondary hover:text-accent"
                >
                  <span className="font-mono text-text-muted">{chapter.number}</span>
                  {chapter.title}
                </a>
              ))}
              <a href="#statuses" className="block py-3 text-xs text-text-muted hover:text-accent">
                Статусы инцидента
              </a>
              <a href="#questions" className="block py-3 text-xs text-text-muted hover:text-accent">
                Частые вопросы
              </a>
            </nav>
          </Card>
          <div className="space-y-3 px-1">
            <Link
              to="/incidents"
              className="flex items-center gap-2 text-xs text-text-secondary hover:text-accent"
            >
              <Search size={14} />
              Найти инцидент
            </Link>
            <Link
              to="/settings/integrations"
              className="flex items-center gap-2 text-xs text-text-secondary hover:text-accent"
            >
              Посмотреть интеграции
            </Link>
            <a
              href="https://github.com/sersert/IncidentGPT/blob/main/docs/getting-started.md"
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-2 text-xs text-text-secondary hover:text-accent"
            >
              <BookOpen size={14} />
              Инструкция администратора
            </a>
          </div>
        </aside>
      </div>
    </div>
  )
}
