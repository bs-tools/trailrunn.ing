import type { LactateStage, LactateTest } from "gen/lactate/v1/lactate_pb";
import { ThresholdDefinition } from "gen/lactate/v1/lactate_pb";

/**
 * DEFAULT_START_HEART_RATE is the heart rate of a guided test's first stage,
 * unless the last test started elsewhere.
 */
export const DEFAULT_START_HEART_RATE = 100;

// DEFAULT_END_HEART_RATE is the heart rate of a Garmin workout's last stage.
export const DEFAULT_END_HEART_RATE = 180;

/**
 * DEFAULT_RANGE_BPM is how far the heart rate may stray from a stage's on a
 * Garmin workout, e.g. 95 to 105 bpm for 100.
 */
export const DEFAULT_RANGE_BPM = 5;

// DEFAULT_STEP_BPM is how much higher each stage's heart rate is.
export const DEFAULT_STEP_BPM = 10;

/**
 * DEFAULT_STAGE_MINUTES is how long each stage is held: at least 3 minutes,
 * for the lactate to settle.
 */
export const DEFAULT_STAGE_MINUTES = 4;

// WARM_UP_MINUTES is how long to warm up before the first stage.
export const WARM_UP_MINUTES = 15;

/**
 * STOP_ABOVE_MMOL is the lactate past which a test has found every
 * definition of the aerobic threshold, together with lowest + 1 mmol/L.
 */
const STOP_ABOVE_MMOL = 2.5;

/**
 * LOWEST_PLUS_MMOL is how far above the lowest reading the baseline-relative
 * definition is.
 */
const LOWEST_PLUS_MMOL = 1;

// FormatMmol formats a lactate reading, e.g. "1.4 mmol/L".
export const formatMmol = (mmolPerLiter: number): string =>
  `${mmolPerLiter.toFixed(1)} mmol/L`;

// DefinitionLabel names a definition of the aerobic or anaerobic threshold.
export const definitionLabel = (definition: ThresholdDefinition): string => {
  switch (definition) {
    case ThresholdDefinition.FIXED_2_MMOL: {
      return "2 mmol/L";
    }
    case ThresholdDefinition.FIXED_2_5_MMOL: {
      return "2.5 mmol/L";
    }
    case ThresholdDefinition.LOWEST_PLUS_1_MMOL: {
      return "Lowest + 1 mmol/L";
    }
    case ThresholdDefinition.FIXED_4_MMOL: {
      return "4 mmol/L";
    }
    case ThresholdDefinition.DMAX: {
      return "Dmax";
    }
    default: {
      return "Unknown";
    }
  }
};

// AerobicThreshold is the user's current aerobic threshold, from a test.
export interface AerobicThreshold {
  heartRateBpm: number;
  test: LactateTest;
}

/**
 * LatestThreshold returns the aerobic threshold of the latest test that
 * found one, from tests listed the latest first, or undefined if none did.
 */
export const latestThreshold = (
  tests: LactateTest[]
): AerobicThreshold | undefined => {
  const test = tests.find(
    (candidate) => candidate.aerobicThresholdHeartRateBpm > 0
  );
  return test === undefined
    ? undefined
    : { heartRateBpm: test.aerobicThresholdHeartRateBpm, test };
};

/**
 * LowestReading returns the lowest reading at rest or of the stages, or 0 if
 * there are none, like the server's lowest_lactate_mmol_per_liter.
 */
const lowestReading = (resting: number, stages: LactateStage[]): number => {
  let lowest = resting;
  for (const stage of stages) {
    if (lowest === 0 || stage.lactateMmolPerLiter < lowest) {
      lowest = stage.lactateMmolPerLiter;
    }
  }
  return lowest;
};

/**
 * PastThreshold reports whether a test's last stage is past every
 * definition of the aerobic threshold, so the test can stop: at or above 2.5
 * mmol/L, and 1 mmol/L above the lowest reading.
 */
export const pastThreshold = (
  resting: number,
  stages: LactateStage[]
): boolean => {
  const last = stages.at(-1);
  if (last === undefined) {
    return false;
  }
  const target = Math.max(
    STOP_ABOVE_MMOL,
    lowestReading(resting, stages) + LOWEST_PLUS_MMOL
  );
  return last.lactateMmolPerLiter >= target;
};

/**
 * How a test is run around the expected aerobic threshold, the same as the
 * server's stages for watches, in internal/plans/lactate_stages.go.
 *
 * FINE_STEP_BPM is how far apart the stages near the expected threshold
 * are, FINE_WINDOW_BPM how far either side of it they are that close, and
 * START_BELOW_THRESHOLD_BPM how far below it a test starts: three stages of
 * baseline before the close ones.
 */
export const FINE_STEP_BPM = 5;
export const FINE_WINDOW_BPM = 10;
const START_BELOW_THRESHOLD_BPM = 30;

// StagePlan is where a test's stages are.
export interface StagePlan {
  startHeartRate: number;
  stepBpm: number;
  // The expected aerobic threshold, like the last test's, or 0 if unknown.
  thresholdHeartRate: number;
}

// RoundedThreshold rounds a threshold to 5 bpm, where a stage is.
const roundedThreshold = (threshold: number): number =>
  Math.round(threshold / FINE_STEP_BPM) * FINE_STEP_BPM;

/**
 * StartBelowThreshold returns the first stage of a test around an expected
 * threshold.
 */
export const startBelowThreshold = (threshold: number): number =>
  roundedThreshold(threshold) - START_BELOW_THRESHOLD_BPM;

/**
 * EndAboveThreshold returns the least a test's last stage should be: a
 * stage past the close ones, in case the threshold rose.
 */
export const endAboveThreshold = (threshold: number, stepBpm: number): number =>
  roundedThreshold(threshold) + FINE_WINDOW_BPM + stepBpm;

/**
 * NextStage returns the heart rate of the stage after the one at a heart
 * rate: a step higher, or a smaller step around the expected threshold. The
 * last stage below the close ones stops where they start.
 */
const nextStage = (heartRate: number, plan: StagePlan): number => {
  if (plan.thresholdHeartRate <= 0) {
    return heartRate + plan.stepBpm;
  }
  const center = roundedThreshold(plan.thresholdHeartRate);
  const low = center - FINE_WINDOW_BPM;
  if (heartRate >= center + FINE_WINDOW_BPM) {
    return heartRate + plan.stepBpm;
  }
  if (heartRate < low) {
    return Math.min(heartRate + plan.stepBpm, low);
  }
  return heartRate + Math.min(plan.stepBpm, FINE_STEP_BPM);
};

/**
 * NextHeartRate returns the heart rate of the stage after the test's
 * stages: the start heart rate, then the stage above the last one.
 */
export const nextHeartRate = (
  stages: LactateStage[],
  plan: StagePlan
): number => {
  const last = stages.at(-1);
  return last === undefined
    ? plan.startHeartRate
    : nextStage(last.heartRateBpm, plan);
};

// StageHeartRates returns the heart rates of a test's stages, up to an end.
export const stageHeartRates = (plan: StagePlan, end: number): number[] => {
  const heartRates: number[] = [];
  // A step of 0 would never reach the end.
  if (plan.stepBpm < 1) {
    return heartRates;
  }
  for (
    let heartRate = plan.startHeartRate;
    heartRate <= end;
    heartRate = nextStage(heartRate, plan)
  ) {
    heartRates.push(heartRate);
  }
  return heartRates;
};
