import { Check, Copy } from "lucide-react";
import { useState } from "react";

type CopyButtonProps = {
  value: string;
  copyLabel: string;
  copiedLabel: string;
};

export function CopyButton({ value, copyLabel, copiedLabel }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      if (navigator.clipboard) {
        await navigator.clipboard.writeText(value);
      } else {
        const area = document.createElement("textarea");
        area.value = value;
        area.style.position = "fixed";
        area.style.opacity = "0";
        document.body.appendChild(area);
        area.focus();
        area.select();
        document.execCommand("copy");
        document.body.removeChild(area);
      }
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      setCopied(false);
    }
  }

  return (
    <button className="icon-text" type="button" onClick={() => void copy()} aria-label={copied ? copiedLabel : copyLabel}>
      {copied ? <Check size={16} /> : <Copy size={16} />}
      <span>{copied ? copiedLabel : copyLabel}</span>
    </button>
  );
}
