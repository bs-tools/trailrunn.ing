import Card from "@components/ui/card";
import { SelectField } from "@components/ui/fields";
import { ErrorText } from "@components/ui/query-error";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { ActivityService } from "gen/activities/v1/activities_pb";
import {
  getCurrentUser,
  setTimeZone,
} from "gen/users/v1/users-UserService_connectquery";
import { UserService } from "gen/users/v1/users_pb";
import { invalidateServices } from "helpers/query";
import React, { useEffect, useRef } from "react";

/**
 * BrowserTimeZone returns the browser's time zone, an IANA name, or
 * undefined if it has none, as some report "UTC" or an offset when they
 * can't tell.
 */
const browserTimeZone = (): string | undefined => {
  const zone = new Intl.DateTimeFormat().resolvedOptions().timeZone;
  return zone.includes("/") ? zone : undefined;
};

/**
 * TimeZoneOptions lists the time zones to choose from: the browser's own
 * list, with the current one, which an older browser may not know.
 */
const timeZoneOptions = (current: string): string[] => {
  const zones = new Set(Intl.supportedValuesOf("timeZone"));
  if (current !== "") {
    zones.add(current);
  }
  const sorted = [...zones].filter((zone) => zone.includes("/"));
  sorted.sort((left, right) => left.localeCompare(right));
  return sorted;
};

/**
 * UseDefaultTimeZone sets the time zone the user lives in to the browser's
 * when it isn't set, so the calendar feed and activities without a GPS
 * track have one without the user doing anything. Once set, it only changes
 * in the settings: a trip abroad shouldn't move the user's workouts.
 */
export const useDefaultTimeZone = (timeZone: string | undefined): void => {
  const queryClient = useQueryClient();
  const tried = useRef(false);
  const { mutate } = useMutation(setTimeZone, {
    onSuccess: async () => {
      await invalidateServices(queryClient, [UserService, ActivityService]);
    },
  });
  useEffect(() => {
    const zone = browserTimeZone();
    // Once per page load, also if it fails: the next load tries again.
    if (timeZone !== "" || zone === undefined || tried.current) {
      return;
    }
    tried.current = true;
    mutate({ timeZone: zone });
  }, [timeZone, mutate]);
};

/**
 * TimeZoneCard sets the time zone the user lives in, which the start times
 * of their workouts are in, and their activities without a GPS track.
 */
const TimeZoneCard: React.FC = () => {
  const queryClient = useQueryClient();
  const { data } = useQuery(getCurrentUser);
  const save = useMutation(setTimeZone, {
    onSuccess: async () => {
      await invalidateServices(queryClient, [UserService, ActivityService]);
    },
  });
  const current = data?.user?.timeZone ?? "";
  return (
    <Card title="Time zone">
      <div className="flex flex-col gap-3">
        <p className="text-sm text-gray-600">
          Where you live. The start times of your workouts are in this time
          zone, on your calendar feed too, and so are activities without GPS,
          like strength sessions indoors, whatever your watch thinks. Activities
          with GPS keep the time zone of where they were.
        </p>
        <SelectField
          label="Time zone"
          value={current}
          onChange={(timeZone) => {
            save.mutate({ timeZone });
          }}
          options={[
            ...(current === "" ? [{ label: "Not set", value: "" }] : []),
            ...timeZoneOptions(current).map((zone) => ({
              label: zone.replaceAll("_", " "),
              value: zone,
            })),
          ]}
        />
        <ErrorText error={save.error} />
        {save.isPending && (
          <span className="text-sm text-gray-600">Saving…</span>
        )}
        {save.isSuccess && (
          <span className="text-sm text-gray-600">Saved.</span>
        )}
      </div>
    </Card>
  );
};

export default TimeZoneCard;
