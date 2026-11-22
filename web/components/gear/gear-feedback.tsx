import { create } from "@bufbuild/protobuf";
import Card from "@components/ui/card";
import { TextAreaField } from "@components/ui/fields";
import { ErrorText } from "@components/ui/query-error";
import { primaryButton } from "@components/ui/styles";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import type { GearComfort, GearFeedback } from "gen/plans/v1/gear_pb";
import { GearArea, GearAreaFeelSchema, GearFeel } from "gen/plans/v1/gear_pb";
import {
  getGearFeedback,
  setGearFeedback,
} from "gen/plans/v1/plans-TrainingPlanService_connectquery";
import { TrainingPlanService } from "gen/plans/v1/plans_pb";
import { invalidateServices } from "helpers/query";
import React, { useState } from "react";

// The parts of the body to rate, in order.
const AREAS: { area: GearArea; label: string }[] = [
  { area: GearArea.LEGS, label: "Legs" },
  { area: GearArea.BODY, label: "Body" },
  { area: GearArea.HANDS, label: "Hands" },
  { area: GearArea.HEAD, label: "Head" },
];

const FEELS: { feel: GearFeel; label: string; active: string }[] = [
  {
    active: "bg-sky-600 text-white ring-sky-600",
    feel: GearFeel.TOO_COLD,
    label: "Too cold",
  },
  {
    active: "bg-emerald-600 text-white ring-emerald-600",
    feel: GearFeel.JUST_RIGHT,
    label: "Just right",
  },
  {
    active: "bg-orange-600 text-white ring-orange-600",
    feel: GearFeel.TOO_WARM,
    label: "Too warm",
  },
];

// AreaName names a part of the body in a sentence, e.g. "hands".
export const areaName = (area: GearArea): string =>
  AREAS.find((entry) => entry.area === area)?.label.toLowerCase() ?? "body";

/**
 * FormatComfort describes what feedback taught about a part of the body,
 * e.g. "hands: dressed 3 °C colder".
 */
export const formatComfort = (comfort: GearComfort): string => {
  const offset = comfort.offsetCelsius;
  const name = `${areaName(comfort.area)}${comfort.wet ? " when wet" : ""}`;
  if (offset === 0) {
    return `${name}: as it feels`;
  }
  const direction = offset > 0 ? "colder" : "warmer";
  return `${name}: dressed ${Math.abs(offset)} °C ${direction}`;
};

interface FormProps {
  workoutId: string;
  stored: GearFeedback | undefined;
  // Whether the forecast says it's wet, for new feedback.
  forecastWet: boolean;
}

// Form rates each part of the body, starting from the stored feedback.
const Form: React.FC<FormProps> = ({ workoutId, stored, forecastWet }) => {
  const queryClient = useQueryClient();
  const [feels, setFeels] = useState<Map<GearArea, GearFeel>>(
    () => new Map(stored?.feels.map((rated) => [rated.area, rated.feel]) ?? [])
  );
  const [notes, setNotes] = useState(stored?.notes ?? "");
  const [wet, setWet] = useState(stored?.wet ?? forecastWet);
  const save = useMutation(setGearFeedback, {
    onSuccess: async () => {
      // What to wear on other workouts depends on it.
      await invalidateServices(queryClient, [TrainingPlanService]);
    },
  });
  const comfort = save.data?.comfort.filter(
    (learned) => learned.offsetCelsius !== 0
  );
  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={(event) => {
        event.preventDefault();
        save.mutate({
          feedback: {
            feels: [...feels].map(([area, feel]) =>
              create(GearAreaFeelSchema, { area, feel })
            ),
            notes,
            wet,
          },
          workoutId,
        });
      }}
    >
      <div className="flex flex-col gap-2">
        {AREAS.map(({ area, label }) => (
          <div
            key={area}
            role="group"
            aria-label={label}
            className="flex flex-wrap items-center gap-2"
          >
            <span className="w-14 text-sm font-medium text-gray-700">
              {label}
            </span>
            {FEELS.map(({ feel, label: feelLabel, active }) => {
              const selected = feels.get(area) === feel;
              return (
                <button
                  key={feel}
                  type="button"
                  aria-pressed={selected}
                  className={`rounded-full px-3 py-1 text-xs font-medium ring-1 ${selected ? active : "bg-white text-gray-700 ring-gray-300 hover:bg-gray-50"}`}
                  onClick={() => {
                    const next = new Map(feels);
                    // Pressing the selected feel again clears it.
                    if (selected) {
                      next.delete(area);
                    } else {
                      next.set(area, feel);
                    }
                    setFeels(next);
                  }}
                >
                  {feelLabel}
                </button>
              );
            })}
          </div>
        ))}
      </div>
      <label className="flex items-start gap-2 text-sm text-gray-700">
        <input
          type="checkbox"
          className="mt-1"
          checked={wet}
          onChange={(event) => {
            setWet(event.target.checked);
          }}
        />
        <span>
          It was wet
          <span className="block text-xs text-gray-500">
            Rain or soaking brush. Feedback on wet runs only changes what to
            wear when rain is likely.
          </span>
        </span>
      </label>
      <TextAreaField
        label="Notes"
        value={notes}
        onChange={setNotes}
        placeholder="e.g. Hands cold for the first 20 minutes"
      />
      <ErrorText error={save.error} />
      <div className="flex flex-wrap items-center gap-3">
        <button
          type="submit"
          className={primaryButton}
          disabled={save.isPending}
        >
          {save.isPending ? "Saving…" : "Save"}
        </button>
        {save.isSuccess && (
          <span className="text-sm text-gray-600">
            Saved.
            {comfort !== undefined && comfort.length > 0
              ? ` From now on, ${comfort.map(formatComfort).join(", ")}.`
              : ""}
          </span>
        )}
      </div>
    </form>
  );
};

/**
 * GearFeedbackCard asks how warm each part of the body was on a workout,
 * which adjusts what later workouts suggest to wear.
 */
interface GearFeedbackCardProps {
  workoutId: string;
  // Whether the forecast says it's wet during the workout.
  forecastWet: boolean;
}

const GearFeedbackCard: React.FC<GearFeedbackCardProps> = ({
  workoutId,
  forecastWet,
}) => {
  const { data, isPending } = useQuery(getGearFeedback, { workoutId });
  return (
    <Card title="How did your gear feel?">
      <div className="flex flex-col gap-3">
        <p className="text-sm text-gray-600">
          Too cold dresses that part of you as if it were 1.5 °C colder next
          time, and too warm as if warmer. Just right keeps it.
        </p>
        {!isPending && (
          <Form
            /*
             * Mounted once the stored feedback is loaded, and kept as it's
             * saved and loaded again.
             */
            workoutId={workoutId}
            stored={data?.feedback}
            forecastWet={forecastWet}
          />
        )}
      </div>
    </Card>
  );
};

export default GearFeedbackCard;
