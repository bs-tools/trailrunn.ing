import type { Muscle } from "gen/plans/v1/muscles_pb";
import type { RecoveryItem, RecoveryWorkout } from "gen/plans/v1/plans_pb";
import {
  RecoveryModality,
  RecoverySide,
  StimulationProgram,
  TensMassage,
} from "gen/plans/v1/plans_pb";
import { formatHoursMinutes } from "helpers/format";
import { muscleLabel } from "helpers/strength";

// Labels of recovery work, for planning and showing it.

export const MODALITY_LABELS: { modality: RecoveryModality; label: string }[] =
  [
    {
      label: "Electrical stimulation",
      modality: RecoveryModality.ELECTRICAL_STIMULATION,
    },
    { label: "Massage gun", modality: RecoveryModality.PERCUSSION },
    { label: "Foam rolling", modality: RecoveryModality.FOAM_ROLLING },
    { label: "Massage", modality: RecoveryModality.MASSAGE },
    { label: "Compression", modality: RecoveryModality.COMPRESSION },
    { label: "Heat", modality: RecoveryModality.HEAT },
    { label: "Cold", modality: RecoveryModality.COLD },
    { label: "Other", modality: RecoveryModality.OTHER },
  ];

export const modalityLabel = (modality: RecoveryModality): string =>
  MODALITY_LABELS.find((entry) => entry.modality === modality)?.label ??
  "Recovery";

// StimulationKind groups programs the way the PowerDot app does.
export type StimulationKind = "recovery" | "training" | "wellness" | "pain";

export interface ProgramInfo {
  program: StimulationProgram;
  label: string;
  kind: StimulationKind;
  // What the program does, in a few words.
  purpose: string;
}

// PROGRAMS are the stimulation programs, named as in the PowerDot app.
export const PROGRAMS: ProgramInfo[] = [
  {
    kind: "recovery",
    label: "Active Recovery",
    program: StimulationProgram.ACTIVE_RECOVERY,
    purpose: "Twitches that raise blood flow, within 2 hours of training.",
  },
  {
    kind: "recovery",
    label: "Light Recovery",
    program: StimulationProgram.LIGHT_RECOVERY,
    purpose: "Gentle, non-fatiguing stimulation between hard days.",
  },
  {
    kind: "recovery",
    label: "Extended Recovery",
    program: StimulationProgram.EXTENDED_RECOVERY,
    purpose: "Active Recovery for longer, e.g. after a race.",
  },
  {
    kind: "training",
    label: "Muscle Endurance",
    program: StimulationProgram.MUSCLE_ENDURANCE,
    purpose: "Fatigue resistance.",
  },
  {
    kind: "training",
    label: "Strength Endurance",
    program: StimulationProgram.STRENGTH_ENDURANCE,
    purpose: "Holding intense efforts for longer.",
  },
  {
    kind: "training",
    label: "Resistance",
    program: StimulationProgram.RESISTANCE,
    purpose: "Sustained intense contractions that build muscle.",
  },
  {
    kind: "training",
    label: "Strength",
    program: StimulationProgram.STRENGTH,
    purpose: "Maximal strength.",
  },
  {
    kind: "training",
    label: "Explosive Strength",
    program: StimulationProgram.EXPLOSIVE_STRENGTH,
    purpose: "Rapid force, for power.",
  },
  {
    kind: "training",
    label: "Warm Up / Potentiation",
    program: StimulationProgram.POTENTIATION,
    purpose: "Readies the muscles for maximal efforts.",
  },
  {
    kind: "wellness",
    label: "Massage",
    program: StimulationProgram.MASSAGE,
    purpose: "Rhythmic contractions that relax the muscles.",
  },
  {
    kind: "pain",
    label: "Pain Relief (TENS)",
    program: StimulationProgram.PAIN_RELIEF,
    purpose: "Eases pain without contracting the muscles.",
  },
  {
    kind: "pain",
    label: "Period Pain Relief (TENS)",
    program: StimulationProgram.PERIOD_PAIN_RELIEF,
    purpose: "On the lower abdomen or lower back.",
  },
];

export const KIND_LABELS: Record<StimulationKind, string> = {
  pain: "Pain",
  recovery: "Recovery",
  training: "Training",
  wellness: "Wellness",
};

export const programInfo = (
  program: StimulationProgram
): ProgramInfo | undefined =>
  PROGRAMS.find((entry) => entry.program === program);

// IsTens reports whether a program eases pain with TENS.
export const isTens = (program: StimulationProgram): boolean =>
  programInfo(program)?.kind === "pain";

export const TENS_MASSAGE_LABELS: { massage: TensMassage; label: string }[] = [
  { label: "Refreshing", massage: TensMassage.REFRESHING },
  { label: "Low frequency (7 phases)", massage: TensMassage.LOW_FREQUENCY },
  { label: "Multi-phase", massage: TensMassage.MULTI_PHASE },
];

export const tensMassageLabel = (massage: TensMassage): string =>
  TENS_MASSAGE_LABELS.find((entry) => entry.massage === massage)?.label ?? "";

export const sideLabel = (side: RecoverySide): string => {
  switch (side) {
    case RecoverySide.LEFT: {
      return "Left";
    }
    case RecoverySide.RIGHT: {
      return "Right";
    }
    default: {
      return "";
    }
  }
};

// WhereLabel says where an item is done, e.g. "Left calves, hamstrings".
export const whereLabel = (muscles: Muscle[], side: RecoverySide): string => {
  const names = muscles.map(muscleLabel).join(", ");
  const sideName = sideLabel(side);
  if (sideName === "") {
    return names;
  }
  return names === ""
    ? `${sideName} side`
    : `${sideName} ${names.charAt(0).toLowerCase()}${names.slice(1)}`;
};

/**
 * ItemTitle names a recovery item: its program for electrical stimulation,
 * or else its modality.
 */
export const itemTitle = (item: RecoveryItem): string => {
  if (item.stimulation !== undefined) {
    return programInfo(item.stimulation.program)?.label ?? "Stimulation";
  }
  return modalityLabel(item.modality);
};

// ItemSummary is one line about an item, e.g. "Active Recovery · Calves · 25 min".
export const itemSummary = (item: RecoveryItem): string =>
  [
    itemTitle(item),
    whereLabel(item.muscles, item.side),
    item.durationSeconds > 0 ? formatHoursMinutes(item.durationSeconds) : "",
  ]
    .filter((part) => part !== "")
    .join(" · ");

/**
 * RecoveryDuration is how long recovery work takes: its target, or else its
 * items' durations together.
 */
export const recoveryDuration = (recovery: RecoveryWorkout): number =>
  recovery.targetDurationSeconds > 0
    ? recovery.targetDurationSeconds
    : recovery.items.reduce((total, item) => total + item.durationSeconds, 0);
