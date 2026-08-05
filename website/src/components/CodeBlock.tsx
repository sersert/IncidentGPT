import { CopyButton } from "./CopyButton";

type CodeBlockProps = {
  code: string;
  language?: string;
  title?: string;
  copyLabel: string;
  copiedLabel: string;
};

export function CodeBlock({ code, language = "text", title, copyLabel, copiedLabel }: CodeBlockProps) {
  return (
    <figure className="code-block">
      <figcaption>
        <span>{title ?? language}</span>
        <CopyButton value={code} copyLabel={copyLabel} copiedLabel={copiedLabel} />
      </figcaption>
      <pre>
        <code>{code}</code>
      </pre>
    </figure>
  );
}
