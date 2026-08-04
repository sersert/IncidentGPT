import { describe, expect, it } from "vitest";
import { buildBasePath } from "../lib/basePath";
import { defaultGeneratorValues, generateAiWorkerValues, generateAlertmanagerSnippet, generateEnricherValues } from "../lib/generator";
import { searchDocs } from "../lib/search";

describe("configuration generator", () => {
  it("references existingSecret without requesting real secrets", () => {
    const yaml = generateAiWorkerValues(defaultGeneratorValues);
    expect(yaml).toContain("existingSecret: incidentgpt-ai-worker");
    expect(yaml).toContain("OPENROUTER_TIMEOUT_SECONDS");
    expect(yaml).not.toContain("replace-me");
  });

  it("uses Go template compatible enricher values", () => {
    const yaml = generateEnricherValues(defaultGeneratorValues);
    expect(yaml).toContain('corrSettle: "40s"');
    expect(yaml).toContain("redis-master.incidentgpt.svc.cluster.local:6379");
  });

  it("builds the Alertmanager receiver snippet", () => {
    expect(generateAlertmanagerSnippet()).toContain("http://incidentgpt-enricher.incidentgpt.svc:9099/alert");
  });
});

describe("base path", () => {
  it("uses repository name on GitHub Actions", () => {
    expect(buildBasePath("sersert/IncidentGPT", true)).toBe("/IncidentGPT/");
  });

  it("uses root locally", () => {
    expect(buildBasePath("sersert/IncidentGPT", false)).toBe("/");
  });
});

describe("local search", () => {
  it("finds entries by title and description", () => {
    const results = searchDocs(
      [
        { id: "installation", title: "Installation", description: "Telegram and Redis", anchors: [] },
        { id: "limitations", title: "Limitations", description: "Correlation limits", anchors: [] },
      ],
      "redis",
    );
    expect(results).toHaveLength(1);
    expect(results[0].id).toBe("installation");
  });
});
