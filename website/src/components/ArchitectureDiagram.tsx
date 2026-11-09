import { useEffect, useId, useState } from "react";
import { CodeBlock } from "./CodeBlock";

type ArchitectureDiagramProps = {
  chart: string;
  fallbackTitle: string;
  copyLabel: string;
  copiedLabel: string;
};

export function ArchitectureDiagram({
  chart,
  fallbackTitle,
  copyLabel,
  copiedLabel,
}: ArchitectureDiagramProps) {
  const id = useId().replace(/:/g, "");
  const [svg, setSvg] = useState("");
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    let revision = 0;
    async function render() {
      const current = ++revision;
      try {
        // Diagrams are only needed on the architecture route.
        const { default: mermaid } = await import("mermaid");
        if (cancelled || current !== revision) return;
        const styles = getComputedStyle(document.documentElement);
        const color = (name: string) => styles.getPropertyValue(name).trim();
        mermaid.initialize({
          startOnLoad: false,
          theme: "base",
          securityLevel: "strict",
          themeVariables: {
            primaryColor: color("--card-2"),
            primaryTextColor: color("--text"),
            primaryBorderColor: color("--border"),
            lineColor: color("--muted"),
            secondaryColor: color("--surface"),
            tertiaryColor: color("--bg"),
            noteBkgColor: color("--card-2"),
            noteTextColor: color("--text"),
            actorBkg: color("--card-2"),
            actorTextColor: color("--text"),
            signalColor: color("--muted"),
            signalTextColor: color("--text"),
          },
        });
        const result = await mermaid.render(`diagram-${id}-${current}`, chart);
        if (!cancelled && current === revision) {
          setSvg(result.svg);
          setFailed(false);
        }
      } catch {
        if (!cancelled && current === revision) setFailed(true);
      }
    }
    void render();
    const observer = new MutationObserver(() => {
      void render();
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    return () => {
      observer.disconnect();
      cancelled = true;
    };
  }, [chart, id]);

  if (failed) {
    return (
      <CodeBlock
        code={chart}
        title={fallbackTitle}
        language="mermaid"
        copyLabel={copyLabel}
        copiedLabel={copiedLabel}
      />
    );
  }
  return (
    <div
      className="diagram"
      aria-label={fallbackTitle}
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
