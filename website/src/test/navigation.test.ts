import { describe, expect, it } from "vitest";
import { buildRoute, parseRouteHash } from "../data/navigation";
import { resolveProductUrl } from "../lib/product";

describe("shareable documentation sections", () => {
  it("round trips a section without losing its page or language", () => {
    expect(
      parseRouteHash(buildRoute("ru", "configuration", "generator"), "en"),
    ).toEqual({ language: "ru", path: "configuration", section: "generator" });
  });
  it("keeps old routes and localizes an existing section", () => {
    expect(parseRouteHash("#/en/architecture", "ru")).toEqual({
      language: "en",
      path: "architecture",
      section: "",
    });
    expect(
      parseRouteHash(buildRoute("en", "configuration", "CPU / memory"), "ru")
        .section,
    ).toBe("CPU / memory");
  });
});
describe("instance links", () => {
  it("hides absent or unsafe instance URLs", () => {
    for (const url of [
      undefined,
      "",
      "wrong",
      "javascript:alert(1)",
      "https://user:password@example.com",
    ])
      expect(resolveProductUrl(url)).toBeUndefined();
  });
  it("uses the configured instance including a deployment subpath", () => {
    expect(resolveProductUrl(" https://instance.example/team/ ")).toBe(
      "https://instance.example/team/",
    );
  });
});
