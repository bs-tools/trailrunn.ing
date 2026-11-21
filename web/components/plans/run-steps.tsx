import { clone } from "@bufbuild/protobuf";
import type { HeartRateChoice } from "@components/plans/heart-rate";
import {
  choiceOf,
  formatHeartRate,
  HeartRateField,
  targetOf,
} from "@components/plans/heart-rate";
import Card from "@components/ui/card";
import { ErrorText } from "@components/ui/query-error";
import {
  ghostButton,
  primaryButton,
  secondaryButton,
} from "@components/ui/styles";
import type { ConnectError } from "@connectrpc/connect";
import { HeartIcon } from "@heroicons/react/24/outline";
import type { HeartRateTarget, RunBlock, RunStep } from "gen/plans/v1/plans_pb";
import { RunBlockSchema, RunStepKind } from "gen/plans/v1/plans_pb";
import { formatPace } from "helpers/format";
import React, { useState } from "react";

const KIND_LABELS: Record<RunStepKind, string> = {
  [RunStepKind.UNSPECIFIED]: "Step",
  [RunStepKind.WARM_UP]: "Warm-up",
  [RunStepKind.RUN]: "Run",
  [RunStepKind.RECOVERY]: "Recovery",
  [RunStepKind.REST]: "Rest",
  [RunStepKind.COOL_DOWN]: "Cool-down",
};

const KIND_DOTS: Record<RunStepKind, string> = {
  [RunStepKind.UNSPECIFIED]: "bg-gray-400",
  [RunStepKind.WARM_UP]: "bg-amber-400",
  [RunStepKind.RUN]: "bg-red-600",
  [RunStepKind.RECOVERY]: "bg-sky-500",
  [RunStepKind.REST]: "bg-gray-400",
  [RunStepKind.COOL_DOWN]: "bg-emerald-500",
};

// FormatStepDuration formats how long a step lasts, e.g. "3 min" or "400 m".
const formatStepDuration = (step: RunStep): string => {
  if (step.durationSeconds > 0) {
    const minutes = Math.floor(step.durationSeconds / 60);
    const seconds = step.durationSeconds % 60;
    if (seconds === 0) {
      return `${minutes} min`;
    }
    return minutes === 0
      ? `${seconds} s`
      : `${minutes}:${String(seconds).padStart(2, "0")} min`;
  }
  if (step.distanceMeters > 0) {
    return step.distanceMeters >= 1000
      ? `${(step.distanceMeters / 1000).toFixed(1)} km`
      : `${Math.round(step.distanceMeters)} m`;
  }
  return "Until lap";
};

// FormatTarget formats a step's target, empty if it has none.
const formatTarget = (step: RunStep): string => {
  switch (step.target.case) {
    case "heartRate": {
      return formatHeartRate(step.target.value);
    }
    case "pace": {
      return `${formatPace(step.target.value.fastSecondsPerKilometer).replace(" /km", "")}–${formatPace(step.target.value.slowSecondsPerKilometer)}`;
    }
    default: {
      return "";
    }
  }
};

interface StepProps {
  step: RunStep;
  // Sets the step's heart rate range, if it can be changed.
  onHeartRate: ((target: HeartRateTarget | undefined) => void) | undefined;
  pending: boolean;
}

const Step: React.FC<StepProps> = ({ step, onHeartRate, pending }) => {
  const target = formatTarget(step);
  const stored =
    step.target.case === "heartRate" ? step.target.value : undefined;
  // The choice being edited, undefined while the step is only shown.
  const [choice, setChoice] = useState<HeartRateChoice>();
  const [invalid, setInvalid] = useState("");
  return (
    <li className="flex items-start gap-2 py-1.5 text-sm">
      <span
        className={`mt-1.5 inline-block size-2 shrink-0 rounded-full ${KIND_DOTS[step.kind]}`}
      />
      <div className="min-w-0 grow">
        <p className="text-gray-900">
          <span className="font-medium">{KIND_LABELS[step.kind]}</span>{" "}
          <span className="text-gray-700 tabular-nums">
            {formatStepDuration(step)}
          </span>
          {target !== "" && (
            <span className="text-gray-600 tabular-nums"> · {target}</span>
          )}
        </p>
        {step.notes !== "" && (
          <p className="text-xs text-gray-600">{step.notes}</p>
        )}
        {choice !== undefined && onHeartRate !== undefined && (
          <form
            className="mt-2 flex flex-col gap-2"
            onSubmit={(event) => {
              event.preventDefault();
              const result = targetOf(choice);
              if ("error" in result) {
                setInvalid(result.error);
                return;
              }
              setInvalid("");
              onHeartRate(result.target);
              setChoice(undefined);
            }}
          >
            <HeartRateField
              label="Heart rate"
              defaultLabel="No range"
              value={choice}
              onChange={setChoice}
            />
            {invalid !== "" && (
              <p className="text-xs text-red-700">{invalid}</p>
            )}
            <div className="flex gap-2">
              <button type="submit" className={primaryButton}>
                Save
              </button>
              <button
                type="button"
                className={secondaryButton}
                onClick={() => {
                  setChoice(undefined);
                  setInvalid("");
                }}
              >
                Cancel
              </button>
            </div>
          </form>
        )}
      </div>
      {onHeartRate !== undefined && choice === undefined && (
        <button
          type="button"
          className={ghostButton}
          disabled={pending}
          aria-label={`Set the heart rate of ${KIND_LABELS[step.kind].toLowerCase()} ${formatStepDuration(step)}`}
          onClick={() => {
            setChoice(choiceOf(stored));
          }}
        >
          <HeartIcon className="size-4" />
        </button>
      )}
    </li>
  );
};

interface RunStepsProps {
  blocks: RunBlock[];
  /*
   * Saves the blocks after a step's heart rate range was changed. Without
   * it the structure is only shown, as on a shared workout.
   */
  onChange?: ((blocks: RunBlock[]) => void) | undefined;
  pending?: boolean | undefined;
  error?: ConnectError | null | undefined;
}

/**
 * WithHeartRate returns the blocks with the heart rate range of a step
 * set, or removed. Removing it leaves a pace target as it is.
 */
const withHeartRate = (
  blocks: RunBlock[],
  blockIndex: number,
  stepIndex: number,
  target: HeartRateTarget | undefined
): RunBlock[] =>
  blocks.map((block, index) => {
    if (index !== blockIndex) {
      return block;
    }
    const copy = clone(RunBlockSchema, block);
    const step = copy.steps[stepIndex];
    if (step === undefined) {
      return block;
    }
    if (target !== undefined) {
      step.target = { case: "heartRate", value: target };
    } else if (step.target.case === "heartRate") {
      step.target = { case: undefined };
    }
    return copy;
  });

/**
 * RunSteps shows the structure of a run: its steps in order, with the
 * blocks done more than once grouped with how often. With onChange, each
 * step's heart rate range can be set.
 */
const RunSteps: React.FC<RunStepsProps> = ({
  blocks,
  onChange,
  pending = false,
  error,
}) => {
  const steps = (block: RunBlock, blockIndex: number) =>
    block.steps.map((step, stepIndex) => (
      <Step
        // The steps are only listed, never reordered.
        // eslint-disable-next-line react/no-array-index-key
        key={stepIndex}
        step={step}
        pending={pending}
        onHeartRate={
          onChange === undefined
            ? undefined
            : (target) => {
                onChange(withHeartRate(blocks, blockIndex, stepIndex, target));
              }
        }
      />
    ));
  return (
    <Card title="Structure">
      <ol className="flex flex-col gap-1">
        {blocks.map((block, index) =>
          block.repeats > 1 ? (
            // eslint-disable-next-line react/no-array-index-key
            <li key={index} className="rounded-md bg-gray-50 px-2 py-1">
              <p className="text-xs font-semibold text-gray-700">
                {block.repeats} ×
              </p>
              <ul className="divide-y divide-gray-100">
                {steps(block, index)}
              </ul>
            </li>
          ) : (
            // eslint-disable-next-line react/no-array-index-key
            <li key={index}>
              <ul className="divide-y divide-gray-100">
                {steps(block, index)}
              </ul>
            </li>
          )
        )}
      </ol>
      {error !== undefined && error !== null && <ErrorText error={error} />}
    </Card>
  );
};

export default RunSteps;
