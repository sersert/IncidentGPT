import { test, expect } from "@playwright/test";
import { mockDashboardStats } from "../src/api/mocks";

const profiles = [
  {
    id: "alpha",
    label: "Alpha",
    model: "fixture/alpha",
    prompt_version: "evidence-analysis/v2",
    max_tokens: 512,
    revision: "p1",
  },
  {
    id: "beta",
    label: "Beta",
    model: "fixture/beta",
    prompt_version: "evidence-analysis/alternatives-v1",
    max_tokens: 768,
    revision: "p2",
  },
];
const snapshot = {
  case_id: "case-1",
  name: "Checkout OOM",
  source: "manual",
  created_by: "operator",
  created_at: "2026-01-01T12:00:00Z",
  snapshot_hash: "snapshot-v1",
  reference: {
    cause: "Container memory exhaustion",
    kind: "engineer_reference",
    actor: "operator",
  },
  input: {
    evidence: [
      {
        evidence_id: "E1",
        source_type: "test_case",
        availability: "available",
        excerpt: "Container terminated: OOMKilled",
      },
    ],
  },
};

test("case library, explicit multi-model run, comparison and human review", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const launches: Record<string, unknown>[] = [];
  const reviews: Record<string, unknown>[] = [];
  let created = false;
  let revision = 0;
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace("/api/v1", "");
    let json: unknown;
    if (path === "/auth/session") json = { username: "operator" };
    else if (path === "/dashboard/stats") json = mockDashboardStats;
    else if (path === "/predictions") json = [];
    else if (path === "/operations/ai-control")
      json = { enabled: true, revision: 1 };
    else if (path === "/evaluation/config") json = { enabled: true, profiles };
    else if (path === "/evaluation/cases") {
      if (request.method() === "POST") {
        const body = request.postDataJSON();
        expect(body.scenario).toContain("OOMKilled");
        expect(body.expected_cause).toBeTruthy();
        created = true;
        return route.fulfill({ status: 201, json: snapshot });
      }
      json = {
        items: created ? [{ ...snapshot, has_reference: true }] : [],
        has_more: false,
      };
    } else if (path === "/evaluation/cases/case-1") json = snapshot;
    else if (path === "/evaluation/cases/case-1/runs") {
      if (request.method() === "POST") {
        const body = request.postDataJSON();
        launches.push(body);
        if (launches.length === 1)
          return route.fulfill({
            status: 503,
            body: "temporary scheduling failure",
          });
        expect(body.request_id).toBe(launches[0].request_id);
        return route.fulfill({ status: 202, json: { run_id: "run-1" } });
      }
      json = {
        items:
          launches.length > 1
            ? [{ run_id: "run-1", created_at: snapshot.created_at, profiles }]
            : [],
      };
    } else if (path === "/evaluation/runs/run-1")
      json = {
        run: {
          run_id: "run-1",
          test_case: snapshot,
          created_at: snapshot.created_at,
          created_by: "operator",
          profiles,
        },
        paused: false,
        results: profiles.map((p) => ({
          job_id: "run-1:" + p.id,
          status: "complete",
          profile: p,
          review: {
            revision,
            verdict: revision ? "partial" : "unrated",
            note: "",
          },
          result: {
            summary:
              p.id === "alpha"
                ? "Alpha: inspect memory limit"
                : "Beta: compare node pressure",
            model: p.model,
            prompt_version: p.prompt_version,
            started_at: snapshot.created_at,
            completed_at: "2026-01-01T12:00:01Z",
            hypotheses: [
              {
                text: "Memory limit reached",
                supporting_evidence_ids: ["E1"],
                contradicting_evidence_ids: [],
              },
            ],
            missing_information: ["Node state"],
            suggested_next_checks: ["Review node conditions"],
            ...(p.id === "alpha"
              ? { usage: { total_tokens: 120, cost: 0.00012 } }
              : {}),
          },
        })),
      };
    else if (path === "/evaluation/runs/run-1/results/alpha/review") {
      reviews.push(request.postDataJSON());
      revision++;
      json = { revision };
    } else
      return route.fulfill({ status: 404, body: "Unexpected route " + path });
    return route.fulfill({ json });
  });
  await page.goto("/evaluation");
  await expect(
    page.getByRole("heading", { name: "Тестирование моделей", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Кейсов пока нет.", { exact: false }),
  ).toBeVisible();
  await page.getByText("Создать тестовый кейс", { exact: true }).click();
  await page.getByRole("button", { name: "Заполнить учебный пример" }).click();
  await page
    .getByRole("button", { name: "Сохранить кейс", exact: true })
    .click();
  await expect(page).toHaveURL(/case=case-1/);
  await expect(
    page.getByRole("button", { name: "Запустить сравнение · 0" }),
  ).toBeDisabled();
  expect(launches).toHaveLength(0);
  await page.getByRole("checkbox", { name: /Alpha fixture\/alpha/ }).check();
  await page.getByRole("checkbox", { name: /Beta fixture\/beta/ }).check();
  await expect(
    page.getByRole("button", { name: "Запустить сравнение · 2" }),
  ).toBeDisabled();
  await page.getByRole("checkbox", { name: /Разрешаю 2 запрос/ }).check();
  await page.getByRole("button", { name: "Запустить сравнение · 2" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "temporary scheduling failure",
  );
  await page.getByRole("button", { name: "Запустить сравнение · 2" }).click();
  await expect(page).toHaveURL(/run=run-1/);
  expect(launches).toHaveLength(2);
  expect(launches[1]).toMatchObject({
    profile_ids: ["alpha", "beta"],
    confirm_provider_calls: true,
  });
  await expect(
    page.getByText("Alpha: inspect memory limit", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Beta: compare node pressure", { exact: true }),
  ).toBeVisible();
  const region = page.getByRole("region", { name: "Результаты сравнения" });
  await expect(
    region.getByText("Container memory exhaustion", { exact: true }),
  ).toBeVisible();
  await expect(region.getByText("Нет данных", { exact: true })).toHaveCount(2);
  await page
    .getByLabel("Оценка: Alpha", { exact: true })
    .selectOption("partial");
  await region
    .getByRole("button", { name: "Сохранить оценку", exact: true })
    .first()
    .click();
  await expect.poll(() => reviews.length).toBe(1);
  expect(reviews[0]).toMatchObject({ revision: 0, verdict: "partial" });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  expect(errors).toEqual([]);
});

test("disabled and unavailable lab do not show runnable controls", async ({
  page,
}) => {
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/auth/session"))
      return route.fulfill({ json: { username: "operator" } });
    if (path.endsWith("/evaluation/config"))
      return route.fulfill({ json: { enabled: false, profiles: [] } });
    return route.fulfill({
      json: path.endsWith("/dashboard/stats") ? mockDashboardStats : [],
    });
  });
  await page.goto("/evaluation");
  await expect(
    page.getByRole("heading", { name: "Лаборатория ещё не включена" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: /Запустить сравнение/ }),
  ).toHaveCount(0);
  await page.route("**/api/v1/evaluation/config", (route) =>
    route.fulfill({ status: 503, body: "profiles unavailable" }),
  );
  await page.reload();
  await expect(page.getByRole("alert")).toContainText("profiles unavailable");
  await expect(
    page.getByRole("button", { name: /Запустить сравнение/ }),
  ).toHaveCount(0);
});
