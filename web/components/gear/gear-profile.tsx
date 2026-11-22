import Card from "@components/ui/card";
import { TextField } from "@components/ui/fields";
import { ErrorText } from "@components/ui/query-error";
import { primaryButton } from "@components/ui/styles";
import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import type { Gear, GearProfile } from "gen/plans/v1/gear_pb";
import { setGearProfile } from "gen/plans/v1/plans-TrainingPlanService_connectquery";
import { TrainingPlanService } from "gen/plans/v1/plans_pb";
import { invalidateServices } from "helpers/query";
import React, { useState } from "react";

import { GEAR_GROUPS, gearLabel } from "./gear-labels";

// DEFAULT_SHORTS_CELSIUS is the server's default temperature for shorts.
const DEFAULT_SHORTS_CELSIUS = 10;

interface GearProfileCardProps {
  profile: GearProfile | undefined;
}

/**
 * GearProfileCard sets the running and touring gear the user has, and the
 * temperature they run in shorts from, which workouts suggest what to wear
 * and carry with.
 */
const GearProfileCard: React.FC<GearProfileCardProps> = ({ profile }) => {
  const queryClient = useQueryClient();
  const [owned, setOwned] = useState<Set<Gear>>(
    () => new Set(profile?.gear ?? [])
  );
  const [shorts, setShorts] = useState(
    profile?.shortsMinApparentCelsius === undefined
      ? ""
      : String(profile.shortsMinApparentCelsius)
  );
  const [invalid, setInvalid] = useState(false);
  const save = useMutation(setGearProfile, {
    onSuccess: async () => {
      // What to wear on workouts depends on it.
      await invalidateServices(queryClient, [TrainingPlanService]);
    },
  });
  return (
    <Card title="Running and touring gear">
      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault();
          const trimmed = shorts.trim().replace(",", ".");
          const value = Number(trimmed);
          if (trimmed !== "" && !Number.isFinite(value)) {
            setInvalid(true);
            return;
          }
          setInvalid(false);
          save.mutate({
            profile: {
              gear: [...owned],
              shortsMinApparentCelsius: trimmed === "" ? undefined : value,
            },
          });
        }}
      >
        <p className="text-sm text-gray-600">
          Workouts suggest what to wear and carry from the forecast, picking
          from what you have here. Until you tick anything, any gear is
          suggested.
        </p>
        {GEAR_GROUPS.map((group) => (
          <fieldset key={group.title} className="flex flex-col gap-1.5">
            <legend className="mb-1 text-sm font-medium text-gray-900">
              {group.title}
            </legend>
            <div className="grid grid-cols-2 gap-x-4 gap-y-1.5 sm:grid-cols-3">
              {group.gear.map((gear) => (
                <label
                  key={gear}
                  className="flex items-center gap-2 text-sm text-gray-700"
                >
                  <input
                    type="checkbox"
                    checked={owned.has(gear)}
                    onChange={(event) => {
                      const next = new Set(owned);
                      if (event.target.checked) {
                        next.add(gear);
                      } else {
                        next.delete(gear);
                      }
                      setOwned(next);
                    }}
                  />
                  {gearLabel(gear)}
                </label>
              ))}
            </div>
          </fieldset>
        ))}
        <div className="max-w-xs">
          <TextField
            label="Shorts from (feels like °C)"
            value={shorts}
            onChange={setShorts}
            inputMode="decimal"
            placeholder={String(DEFAULT_SHORTS_CELSIUS)}
            hint="Below it, tights. Empty for 10 °C."
          />
        </div>
        {invalid && (
          <p className="text-sm text-red-700">
            Enter a temperature, or leave it empty.
          </p>
        )}
        <ErrorText error={save.error} />
        <div className="flex items-center gap-3">
          <button
            type="submit"
            className={primaryButton}
            disabled={save.isPending}
          >
            {save.isPending ? "Saving…" : "Save"}
          </button>
          {save.isSuccess && (
            <span className="text-sm text-gray-600">Saved.</span>
          )}
        </div>
      </form>
    </Card>
  );
};

export default GearProfileCard;
