import mermaid from "mermaid";
import { useEffect, useId, useState } from "react";
import { CodeBlock } from "./CodeBlock";

type ArchitectureDiagramProps = {
  chart: string;
  fallbackTitle: string;
  copyLabel: string;
  copiedLabel: string;
};

export function ArchitectureDiagram({ chart, fallbackTitle, copyLabel, copiedLabel }: ArchitectureDiagramProps) {
  const id = useId().replace(/:/g, "");
  const [svg, setSvg] = useState<string>("");
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    mermaid.initialize({
      startOnLoad: false,
      theme: document.documentElement.dataset.theme === "light" ? "default" : "dark",
      securityLevel: "strict",
    });
    mermaid
      .render(`diagram-${id}`, chart)
      .then((result) => {
        if (!cancelled) {
          setSvg(result.svg);
          setFailed(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setFailed(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [chart, id]);

  if (failed) {
    return <CodeBlock code={chart} title={fallbackTitle} language="mermaid" copyLabel={copyLabel} copiedLabel={copiedLabel} />;
  }

  return <div className="diagram" aria-label={fallbackTitle} dangerouslySetInnerHTML={{ __html: svg }} />;
}
