import { useId, useRef, useState } from "react";
import {
  ArrowUpRight,
  Check,
  CircleDot,
  GitBranch,
  Layers,
  FileText,
} from "lucide-react";
import type { Translation } from "../i18n/types";
import { BrandMark } from "./BrandMark";

/** An explicitly illustrative, local interaction. Never calls a product API. */
export function IncidentPreview({ t }: { t: Translation }) {
  const [active, setActive] = useState(0);
  const id = useId();
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const d = t.demo;
  const icons = [CircleDot, GitBranch, Layers, FileText];
  return (
    <figure className="incident-preview">
      <figcaption className="preview-bar">
        <span>
          <BrandMark />
          IncidentGPT
        </span>
        <span className="demo-label">{d.label}</span>
      </figcaption>
      <div className="preview-body">
        <div className="preview-breadcrumb">
          {d.queue}
          <span>/</span>
          <span>INC-2026-0042</span>
        </div>
        <div className="preview-meta">
          <span className="severity">{d.severity}</span>
          <span className="incident-status">
            <span className="status-dot" />
            {d.status}
          </span>
          <span>shop / checkout</span>
        </div>
        <h2>{d.title}</h2>
        <p className="preview-summary">{d.summary}</p>
        <div className="preview-tabs" role="tablist" aria-label={d.tabsLabel}>
          {d.tabs.map((tab, index) => {
            const Icon = icons[index];
            return (
              <button
                key={tab}
                ref={(el) => {
                  tabs.current[index] = el;
                }}
                type="button"
                role="tab"
                id={`${id}-tab-${index}`}
                aria-controls={`${id}-panel`}
                aria-selected={active === index}
                tabIndex={active === index ? 0 : -1}
                onClick={() => setActive(index)}
                onKeyDown={(event) => {
                  let next = active;
                  if (event.key === "ArrowRight")
                    next = (active + 1) % d.tabs.length;
                  else if (event.key === "ArrowLeft")
                    next = (active + d.tabs.length - 1) % d.tabs.length;
                  else if (event.key === "Home") next = 0;
                  else if (event.key === "End") next = d.tabs.length - 1;
                  else return;
                  event.preventDefault();
                  setActive(next);
                  tabs.current[next]?.focus();
                }}
              >
                <Icon size={14} />
                {tab}
              </button>
            );
          })}
        </div>
        <div
          className="preview-panel"
          role="tabpanel"
          id={`${id}-panel`}
          aria-labelledby={`${id}-tab-${active}`}
          tabIndex={0}
        >
          <div className="preview-panel-heading">
            <h3>{d.panels[active].title}</h3>
            <span>{d.evidence}</span>
          </div>
          <p>{d.panels[active].body}</p>
          <ul className="evidence-list">
            {d.panels[active].items.map((item, i) => (
              <li key={item}>
                <span className="evidence-index">0{i + 1}</span>
                <span>{item}</span>
                <Check size={14} />
              </li>
            ))}
          </ul>
        </div>
        <div className="preview-footer">
          <span>{d.footer}</span>
          <ArrowUpRight size={16} aria-hidden="true" />
        </div>
      </div>
      <p className="preview-note">{t.hero.demoNote}</p>
    </figure>
  );
}
