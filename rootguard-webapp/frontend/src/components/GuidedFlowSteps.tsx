import { Fragment } from "react";
import { ChevronRight } from "lucide-react";

/**
 * The 3-step draft -> validate -> activate indicator shared by every guided
 * Unbound settings panel. Previously only guided zones had this (a private
 * `FlowStep` component); extracted so all four draft/preview/activate
 * surfaces show the same progress cue instead of only one of them.
 */
export default function GuidedFlowSteps({ steps }: { steps: { label: string; active: boolean }[] }) {
  return (
    <div className="guided-flow">
      {steps.map((step, index) => (
        <Fragment key={step.label}>
          {index > 0 && <ChevronRight size={16} />}
          <span className={step.active ? "active" : ""}><i>{index + 1}</i>{step.label}</span>
        </Fragment>
      ))}
    </div>
  );
}
