import BodyMap from "@components/strength/body-map";
import Card from "@components/ui/card";
import { ErrorText } from "@components/ui/query-error";
import { primaryButton, secondaryButton } from "@components/ui/styles";
import { useMutation } from "@connectrpc/connect-query";
import { CheckIcon } from "@heroicons/react/24/solid";
import { useQueryClient } from "@tanstack/react-query";
import type { Muscle } from "gen/plans/v1/muscles_pb";
import { updateWorkout } from "gen/plans/v1/plans-TrainingPlanService_connectquery";
import type {
  ElectricalStimulation,
  RecoveryItem,
  RecoveryWorkout,
  Workout,
} from "gen/plans/v1/plans_pb";
import { TrainingPlanService } from "gen/plans/v1/plans_pb";
import { formatHoursMinutes } from "helpers/format";
import { invalidateServices } from "helpers/query";
import {
  itemTitle,
  modalityLabel,
  programInfo,
  tensMassageLabel,
  whereLabel,
} from "helpers/recovery";
import { MUSCLE_LABELS } from "helpers/strength";
import React from "react";

// StimulationSettings lists how to set up electrical stimulation.
const stimulationSettings = (stim: ElectricalStimulation): string[] => {
  const settings: string[] = [];
  if (stim.pods > 0) {
    settings.push(`${stim.pods} ${stim.pods === 1 ? "pod" : "pods"}`);
  }
  if (stim.intensity > 0) {
    settings.push(`intensity ${stim.intensity}`);
  }
  if (stim.painLevel !== undefined) {
    settings.push(`pain level ${stim.painLevel}`);
  }
  const massage = tensMassageLabel(stim.tensMassage);
  if (massage !== "") {
    settings.push(`${massage.toLowerCase()} massage`);
  }
  return settings;
};

// ItemView shows one part of recovery work.
const ItemView: React.FC<{ item: RecoveryItem; index: number }> = ({
  item,
  index,
}) => {
  const where = whereLabel(item.muscles, item.side);
  const settings =
    item.stimulation === undefined ? [] : stimulationSettings(item.stimulation);
  const program =
    item.stimulation === undefined
      ? undefined
      : programInfo(item.stimulation.program);
  return (
    <li className="flex gap-3 py-3">
      <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-teal-100 text-xs font-semibold text-teal-900">
        {index + 1}
      </span>
      <div className="flex min-w-0 grow flex-col gap-0.5">
        <p className="flex flex-wrap items-baseline justify-between gap-x-3">
          <span className="font-medium text-gray-900">{itemTitle(item)}</span>
          {item.durationSeconds > 0 && (
            <span className="text-sm text-gray-700 tabular-nums">
              {formatHoursMinutes(item.durationSeconds)}
            </span>
          )}
        </p>
        {item.stimulation !== undefined && (
          <p className="text-xs text-gray-500">
            {modalityLabel(item.modality)}
            {program !== undefined && ` · ${program.purpose}`}
          </p>
        )}
        {where !== "" && <p className="text-sm text-gray-700">{where}</p>}
        {settings.length > 0 && (
          <p className="text-sm text-gray-700">
            {settings
              .join(" · ")
              .replace(/^./u, (first) => first.toUpperCase())}
          </p>
        )}
        {item.notes !== "" && (
          <p className="text-sm whitespace-pre-line text-gray-600">
            {item.notes}
          </p>
        )}
      </div>
    </li>
  );
};

interface RecoveryWorkoutViewProps {
  workout: Workout;
  recovery: RecoveryWorkout;
  // Whether the viewer can mark it done: not on a shared workout's page.
  canMarkDone?: boolean;
}

/**
 * RecoveryWorkoutView shows recovery work in order, with where it's done
 * on the body, and a button to mark it done, as no activity records it.
 */
const RecoveryWorkoutView: React.FC<RecoveryWorkoutViewProps> = ({
  workout,
  recovery,
  canMarkDone = true,
}) => {
  const queryClient = useQueryClient();
  const markDone = useMutation(updateWorkout, {
    onSuccess: async () => {
      await invalidateServices(queryClient, [TrainingPlanService]);
    },
  });
  const muscles = new Set<Muscle>();
  for (const item of recovery.items) {
    for (const muscle of item.muscles) {
      muscles.add(muscle);
    }
  }
  const setDone = (done: boolean) => {
    markDone.mutate({
      updateMask: { paths: ["recovery.done"] },
      workout: {
        details: { case: "recovery", value: { done } },
        id: workout.id,
      },
    });
  };
  return (
    <>
      {canMarkDone && (
        <div className="flex flex-col gap-1">
          {recovery.done ? (
            <button
              type="button"
              className={`${secondaryButton} h-12 text-base sm:self-start sm:px-6`}
              disabled={markDone.isPending}
              onClick={() => {
                setDone(false);
              }}
            >
              Mark as not done
            </button>
          ) : (
            <button
              type="button"
              className={`${primaryButton} h-12 text-base sm:self-start sm:px-6`}
              disabled={markDone.isPending}
              onClick={() => {
                setDone(true);
              }}
            >
              <CheckIcon className="size-5" />
              Mark as done
            </button>
          )}
          <ErrorText error={markDone.error} />
        </div>
      )}
      <Card title="What to do">
        <ol className="-my-3 divide-y divide-gray-100">
          {recovery.items.map((item, index) => (
            // Items have no IDs; their order is their identity.
            // oxlint-disable-next-line no-array-index-key
            <ItemView key={index} item={item} index={index} />
          ))}
        </ol>
      </Card>
      {muscles.size > 0 && (
        <Card title="Where">
          <BodyMap
            muscles={{
              primary: MUSCLE_LABELS.map((entry) => entry.muscle).filter(
                (muscle) => muscles.has(muscle)
              ),
              secondary: [],
            }}
          />
        </Card>
      )}
    </>
  );
};

export default RecoveryWorkoutView;
