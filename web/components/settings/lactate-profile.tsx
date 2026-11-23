import Loading from "@components/loading";
import Card from "@components/ui/card";
import { TextField } from "@components/ui/fields";
import QueryError, { ErrorText } from "@components/ui/query-error";
import { primaryButton } from "@components/ui/styles";
import type { ConnectError } from "@connectrpc/connect";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import {
  getLactateProfile,
  setLactateProfile,
} from "gen/lactate/v1/lactate-LactateTestService_connectquery";
import type { HeartRateZone, LactateProfile } from "gen/lactate/v1/lactate_pb";
import { LactateTestService } from "gen/lactate/v1/lactate_pb";
import { TrainingPlanService } from "gen/plans/v1/plans_pb";
import { invalidateServices } from "helpers/query";
import React, { useState } from "react";

// HeartRates are what the form saves.
interface HeartRates {
  aerobicThresholdBpm: number | undefined;
  maxBpm: number | undefined;
}

interface FormProps {
  profile: LactateProfile | undefined;
  /*
   * Saving is the card's, as the form starts over when the stored heart
   * rates change, also by its own save.
   */
  saving: boolean;
  saved: boolean;
  error: ConnectError | null;
  onSave: (heartRates: HeartRates) => void;
}

/**
 * ParseHeartRate parses a heart rate field: undefined if it's empty, and
 * null if it isn't a whole number.
 */
const parseHeartRate = (value: string): number | undefined | null => {
  if (value.trim() === "") {
    return undefined;
  }
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null;
};

// Form edits the heart rates, starting from the stored ones.
const Form: React.FC<FormProps> = ({
  profile,
  saving,
  saved,
  error,
  onSave,
}) => {
  const [threshold, setThreshold] = useState(
    profile?.aerobicThresholdHeartRateBpm === undefined
      ? ""
      : String(profile.aerobicThresholdHeartRateBpm)
  );
  const [maxHeartRate, setMaxHeartRate] = useState(
    profile?.maxHeartRateBpm === undefined
      ? ""
      : String(profile.maxHeartRateBpm)
  );
  const [invalid, setInvalid] = useState(false);
  const estimated = profile?.estimatedMaxHeartRateBpm ?? 0;
  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={(event) => {
        event.preventDefault();
        const aerobicThresholdBpm = parseHeartRate(threshold);
        const maxBpm = parseHeartRate(maxHeartRate);
        if (aerobicThresholdBpm === null || maxBpm === null) {
          setInvalid(true);
          return;
        }
        setInvalid(false);
        onSave({ aerobicThresholdBpm, maxBpm });
      }}
    >
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <TextField
          label="Expected aerobic threshold (bpm)"
          value={threshold}
          onChange={setThreshold}
          inputMode="numeric"
          placeholder="e.g. 145"
          hint="Each lactate test updates it. Empty for a test with even steps."
        />
        <TextField
          label="Max heart rate (bpm)"
          value={maxHeartRate}
          onChange={setMaxHeartRate}
          inputMode="numeric"
          placeholder={estimated > 0 ? String(estimated) : "e.g. 190"}
          hint={
            estimated > 0
              ? `Empty to use ${estimated} bpm, the highest of your activities in the past year.`
              : "Empty to estimate it from your activities."
          }
        />
      </div>
      {invalid && (
        <p className="text-sm text-red-700">
          Enter heart rates as whole numbers, or leave a field empty.
        </p>
      )}
      <ErrorText error={error} />
      <div className="flex items-center gap-3">
        <button type="submit" className={primaryButton} disabled={saving}>
          {saving ? "Saving…" : "Save"}
        </button>
        {saved && <span className="text-sm text-gray-600">Saved.</span>}
      </div>
    </form>
  );
};

// ZONE_COLORS mark the zones, from easy to hard.
const ZONE_COLORS = [
  "bg-sky-500",
  "bg-emerald-500",
  "bg-amber-500",
  "bg-red-600",
];

/**
 * Zones shows the heart rate zones the profile makes, or what's needed for
 * them.
 */
const Zones: React.FC<{ profile: LactateProfile | undefined }> = ({
  profile,
}) => {
  const zones: HeartRateZone[] = profile?.zones ?? [];
  if (zones.length === 0) {
    return (
      <p className="text-sm text-gray-600">
        Do a lactate test, or enter your aerobic threshold, to see your heart
        rate zones.
      </p>
    );
  }
  const anaerobic = profile?.anaerobicThresholdHeartRateBpm ?? 0;
  return (
    <div className="flex flex-col gap-2">
      <h3 className="text-sm font-medium text-gray-900">Heart rate zones</h3>
      <ul className="divide-y divide-gray-100 text-sm">
        {zones.map((zone) => (
          <li key={zone.zone} className="flex items-start gap-3 py-1.5">
            <span
              className={`mt-1.5 inline-block size-2.5 shrink-0 rounded-full ${ZONE_COLORS[zone.zone - 1] ?? "bg-gray-400"}`}
            />
            <span className="w-14 shrink-0 font-medium text-gray-900">
              Zone {zone.zone}
            </span>
            <span className="w-24 shrink-0 text-gray-900 tabular-nums">
              {zone.highBpm > 0
                ? `${zone.lowBpm}–${zone.highBpm}`
                : `${zone.lowBpm}+`}{" "}
              bpm
            </span>
            <span className="text-xs text-gray-600">{zone.description}</span>
          </li>
        ))}
      </ul>
      <p className="text-xs text-gray-500">
        Uphill Athlete&apos;s zones: zones 1 and 2 are 80–100% of your aerobic
        threshold&apos;s heart rate, zone 3 goes up to your anaerobic threshold
        {anaerobic > 0
          ? ` (${anaerobic} bpm, at 4 mmol/L in your latest test that reached it)`
          : ", which a test that goes past 4 mmol/L finds"}
        , and zone 4 up to your max heart rate. Workouts&apos; heart rate
        targets and your Garmin workouts use them.
      </p>
    </div>
  );
};

/**
 * LactateProfileCard sets the user's heart rates: where their next lactate
 * test expects their aerobic threshold, which the guided test and the
 * planned tests on their watch have closer stages around, and their max
 * heart rate, with the heart rate zones they make.
 */
const LactateProfileCard: React.FC = () => {
  const queryClient = useQueryClient();
  const { isPending, isError, data, error, refetch } =
    useQuery(getLactateProfile);
  const save = useMutation(setLactateProfile, {
    onSuccess: async () => {
      // Workouts' heart rate zones depend on it.
      await invalidateServices(queryClient, [
        LactateTestService,
        TrainingPlanService,
      ]);
    },
  });
  let content: React.ReactNode = <Loading />;
  if (isError) {
    content = (
      <QueryError
        error={error}
        onRetry={() => {
          void refetch();
        }}
      />
    );
  } else if (!isPending) {
    content = (
      <>
        <Form
          // A test may have set the threshold since the form was filled in.
          key={`${data.profile?.aerobicThresholdHeartRateBpm}-${data.profile?.maxHeartRateBpm}`}
          profile={data.profile}
          saving={save.isPending}
          saved={save.isSuccess}
          error={save.error}
          onSave={({ aerobicThresholdBpm, maxBpm }) => {
            save.mutate({
              profile: {
                aerobicThresholdHeartRateBpm: aerobicThresholdBpm,
                maxHeartRateBpm: maxBpm,
              },
            });
          }}
        />
        <Zones profile={data.profile} />
      </>
    );
  }
  return (
    <Card title="Heart rates and zones">
      <div className="flex flex-col gap-4">
        <p className="text-sm text-gray-600">
          Your next lactate test has stages 5 bpm apart around where it expects
          your aerobic threshold, to find it more precisely, and starts 30 bpm
          below it. Each test that finds your threshold updates it.
        </p>
        {content}
      </div>
    </Card>
  );
};

export default LactateProfileCard;
