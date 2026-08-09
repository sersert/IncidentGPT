import { useState } from "react";
import type { Translation } from "../i18n/types";

type AlertFlowProps = {
  t: Translation;
};

export function AlertFlow({ t }: AlertFlowProps) {
  const [active, setActive] = useState(0);
  const steps = t.architecture.flowSteps;
  const labels = t.architecture.flowLabels;
  const step = steps[active];

  return (
    <section className="flow-explorer" aria-label="Incident flow explorer">
      <div className="flow-tabs" role="tablist">
        {steps.map((item, index) => (
          <button key={item.title} type="button" className={index === active ? "selected" : ""} onClick={() => setActive(index)}>
            {item.title}
          </button>
        ))}
      </div>
      <div className="flow-detail">
        <h3>{step.title}</h3>
        <dl>
          <div>
            <dt>{labels.input}</dt>
            <dd>{step.input}</dd>
          </div>
          <div>
            <dt>{labels.action}</dt>
            <dd>{step.action}</dd>
          </div>
          <div>
            <dt>{labels.output}</dt>
            <dd>{step.output}</dd>
          </div>
          <div>
            <dt>{labels.dependency}</dt>
            <dd>{step.dependency}</dd>
          </div>
          <div>
            <dt>{labels.error}</dt>
            <dd>{step.error}</dd>
          </div>
        </dl>
      </div>
    </section>
  );
}
