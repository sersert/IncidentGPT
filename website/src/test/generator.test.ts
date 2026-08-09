import { describe, expect, it } from "vitest";
import { buildBasePath } from "../lib/basePath";
import { defaultGeneratorValues, generateAlertmanagerSnippet, generateUmbrellaValues } from "../lib/generator";
import { searchDocs } from "../lib/search";

describe("configuration generator", () => {
  it("references existingSecret without requesting real secrets", () => {
    const yaml = generateUmbrellaValues(defaultGeneratorValues);
    expect(yaml).toContain("existingSecret: incidentgpt-ai-worker");
    expect(yaml).toContain("OPENROUTER_TIMEOUT_SECONDS");
    expect(yaml).not.toContain("replace-me");
  });

  it("wires the group path and the analysis callback through backend", () => {
    const yaml = generateUmbrellaValues(defaultGeneratorValues);
    expect(yaml).toContain('corrSettle: "40s"');
    expect(yaml).toContain("http://incidentgpt-backend:8080/api/v1/ingest");
    expect(yaml).toContain("http://incidentgpt-backend:8080/api/v1/incidents/by-group/analysis");
  });

  it("keeps every image on the same release tag", () => {
    const yaml = generateUmbrellaValues(defaultGeneratorValues);
    const components = ["enricher", "ai-worker", "sanitizer", "backend", "ui"];
    for (const component of components) {
      expect(yaml).toContain(`repository: ghcr.io/sersert/incidentgpt-${component}`);
    }
    expect(yaml.match(/tag: "0\.2\.0"/g)).toHaveLength(components.length);
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
