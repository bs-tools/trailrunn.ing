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
import type { LactateProfile } from "gen/lactate/v1/lactate_pb";
import { LactateTestService } from "gen/lactate/v1/lactate_pb";
import { invalidateServices } from "helpers/query";
import React, { useState } from "react";

interface FormProps {
  profile: LactateProfile | undefined;
  /*
   * Saving is the card's, as the form starts over when the stored threshold
   * changes, also by its own save.
   */
  saving: boolean;
  saved: boolean;
  error: ConnectError | null;
  onSave: (heartRateBpm: number | undefined) => void;
}

// Form edits the lactate profile, starting from the stored one.
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
  const [invalid, setInvalid] = useState(false);
  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={(event) => {
        event.preventDefault();
        const parsed = Number(threshold);
        const empty = threshold.trim() === "";
        if (!empty && !(Number.isInteger(parsed) && parsed > 0)) {
          setInvalid(true);
          return;
        }
        setInvalid(false);
        onSave(empty ? undefined : parsed);
      }}
    >
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <TextField
          label="Expected aerobic threshold (bpm)"
          value={threshold}
          onChange={setThreshold}
          inputMode="numeric"
          placeholder="e.g. 145"
          hint="Empty for a test with even steps."
        />
      </div>
      {invalid && (
        <p className="text-sm text-red-700">
          Enter a heart rate as a whole number, or leave the field empty.
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

/**
 * LactateProfileCard sets where the user's next lactate test expects their
 * aerobic threshold, which the guided test and the planned tests on their
 * watch have closer stages around.
 */
const LactateProfileCard: React.FC = () => {
  const queryClient = useQueryClient();
  const { isPending, isError, data, error, refetch } =
    useQuery(getLactateProfile);
  const save = useMutation(setLactateProfile, {
    onSuccess: async () => {
      await invalidateServices(queryClient, [LactateTestService]);
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
      <Form
        // A test may have set it since the form was filled in.
        key={data.profile?.aerobicThresholdHeartRateBpm}
        profile={data.profile}
        saving={save.isPending}
        saved={save.isSuccess}
        error={save.error}
        onSave={(heartRateBpm) => {
          save.mutate({
            profile: { aerobicThresholdHeartRateBpm: heartRateBpm },
          });
        }}
      />
    );
  }
  return (
    <Card title="Lactate tests">
      <div className="flex flex-col gap-3">
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
