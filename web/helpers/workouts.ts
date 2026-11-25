import { create } from "@bufbuild/protobuf";
import { Terrain } from "gen/geo/v1/geo_pb";
import type { Workout } from "gen/plans/v1/plans_pb";
import {
  CrossTrainingSport,
  CrossTrainingWorkoutSchema,
  RaceWorkoutSchema,
  RecoveryWorkoutSchema,
  RestDaySchema,
  RunIntensity,
  RunWorkoutSchema,
  StrengthWorkoutSchema,
  WorkoutType,
} from "gen/plans/v1/plans_pb";

// WorkoutTargets are the targets of a workout, whatever its kind.
export interface WorkoutTargets {
  distanceMeters: number;
  elevationGainMeters: number;
  durationSeconds: number;
  terrain: Terrain;
  // Empty if the workout has no route.
  routeId: string;
  // Whether a run is a lactate test.
  lactateTest: boolean;
  // The sport of cross-training, unspecified for other sports.
  crossTrainingSport: CrossTrainingSport;
}

const NO_TARGETS: WorkoutTargets = {
  crossTrainingSport: CrossTrainingSport.UNSPECIFIED,
  distanceMeters: 0,
  durationSeconds: 0,
  elevationGainMeters: 0,
  lactateTest: false,
  routeId: "",
  terrain: Terrain.UNSPECIFIED,
};

// WorkoutTargets returns the targets of a workout from its details.
export const workoutTargets = (workout: Workout): WorkoutTargets => {
  const { details } = workout;
  switch (details.case) {
    case "run": {
      return {
        ...NO_TARGETS,
        distanceMeters: details.value.targetDistanceMeters,
        durationSeconds: details.value.targetDurationSeconds,
        elevationGainMeters: details.value.targetElevationGainMeters,
        lactateTest: details.value.lactateTest,
        routeId: details.value.routeId,
        terrain: details.value.targetTerrain,
      };
    }
    case "crossTraining": {
      return {
        ...NO_TARGETS,
        crossTrainingSport: details.value.sport,
        distanceMeters: details.value.targetDistanceMeters,
        durationSeconds: details.value.targetDurationSeconds,
        elevationGainMeters: details.value.targetElevationGainMeters,
      };
    }
    case "race": {
      return {
        ...NO_TARGETS,
        distanceMeters: details.value.targetDistanceMeters,
        durationSeconds: details.value.targetDurationSeconds,
        elevationGainMeters: details.value.targetElevationGainMeters,
        routeId: details.value.routeId,
      };
    }
    case "strength":
    case "recovery": {
      return {
        ...NO_TARGETS,
        durationSeconds: details.value.targetDurationSeconds,
      };
    }
    default: {
      return NO_TARGETS;
    }
  }
};

/**
 * IsCompleted reports whether a workout is done: it has an activity, it's a
 * strength workout with a log, or recovery work marked done.
 */
export const isCompleted = (workout: Workout): boolean =>
  workout.completedActivityId !== "" ||
  (workout.details.case === "strength" &&
    workout.details.value.log !== undefined) ||
  (workout.details.case === "recovery" && workout.details.value.done);

// DetailsPath is the update mask path of the details of a workout type.
export const detailsPath = (type: WorkoutType): string => {
  switch (type) {
    case WorkoutType.BASE:
    case WorkoutType.INTENSITY: {
      return "run";
    }
    case WorkoutType.CROSS_TRAINING: {
      return "cross_training";
    }
    case WorkoutType.STRENGTH: {
      return "strength";
    }
    case WorkoutType.RACE: {
      return "race";
    }
    case WorkoutType.RECOVERY: {
      return "recovery";
    }
    default: {
      return "rest";
    }
  }
};

/**
 * WorkoutDetails returns the details of a workout of the type with the
 * targets, keeping the exercises of an existing strength workout, and the
 * items of existing recovery work.
 */
export const workoutDetails = (
  type: WorkoutType,
  targets: WorkoutTargets,
  existing: Workout | undefined
): Workout["details"] => {
  switch (type) {
    case WorkoutType.BASE:
    case WorkoutType.INTENSITY: {
      /*
       * The structure, fueling and heart rate range of an existing run are
       * kept, unless the caller replaces them.
       */
      const existingRun =
        existing?.details.case === "run" ? existing.details.value : undefined;
      return {
        case: "run",
        value: create(RunWorkoutSchema, {
          blocks: existingRun?.blocks ?? [],
          fueling: existingRun?.fueling,
          heartRate: existingRun?.heartRate,
          intensity:
            type === WorkoutType.BASE ? RunIntensity.BASE : RunIntensity.HARD,
          lactateTest: targets.lactateTest,
          routeId: targets.routeId,
          targetDistanceMeters: targets.distanceMeters,
          targetDurationSeconds: targets.durationSeconds,
          targetElevationGainMeters: targets.elevationGainMeters,
          targetTerrain: targets.terrain,
        }),
      };
    }
    case WorkoutType.CROSS_TRAINING: {
      return {
        case: "crossTraining",
        value: create(CrossTrainingWorkoutSchema, {
          sport: targets.crossTrainingSport,
          targetDistanceMeters: targets.distanceMeters,
          targetDurationSeconds: targets.durationSeconds,
          targetElevationGainMeters: targets.elevationGainMeters,
        }),
      };
    }
    case WorkoutType.STRENGTH: {
      const existingStrength =
        existing?.details.case === "strength"
          ? existing.details.value
          : undefined;
      return {
        case: "strength",
        value: create(StrengthWorkoutSchema, {
          exercises: existingStrength?.exercises ?? [],
          groups: existingStrength?.groups ?? [],
          targetDurationSeconds: targets.durationSeconds,
        }),
      };
    }
    case WorkoutType.RACE: {
      return {
        case: "race",
        value: create(RaceWorkoutSchema, {
          routeId: targets.routeId,
          targetDistanceMeters: targets.distanceMeters,
          targetDurationSeconds: targets.durationSeconds,
          targetElevationGainMeters: targets.elevationGainMeters,
        }),
      };
    }
    case WorkoutType.RECOVERY: {
      const existingRecovery =
        existing?.details.case === "recovery"
          ? existing.details.value
          : undefined;
      return {
        case: "recovery",
        value: create(RecoveryWorkoutSchema, {
          done: existingRecovery?.done ?? false,
          items: existingRecovery?.items ?? [],
          targetDurationSeconds: targets.durationSeconds,
        }),
      };
    }
    default: {
      return { case: "rest", value: create(RestDaySchema, {}) };
    }
  }
};

// IsSkiTour reports whether a workout is a ski tour.
export const isSkiTour = (workout: Workout): boolean =>
  workout.details.case === "crossTraining" &&
  workout.details.value.sport === CrossTrainingSport.SKI_TOURING;

// HasRoute reports whether a workout's kind can have a route.
export const hasRoute = (type: WorkoutType): boolean =>
  type === WorkoutType.BASE ||
  type === WorkoutType.INTENSITY ||
  type === WorkoutType.RACE;

// DEFAULT_START_WINDOW_MINUTES is the window of a start time without one.
const DEFAULT_START_WINDOW_MINUTES = 60;

// AddMinutes adds minutes to a local time "HH:MM", wrapping at midnight.
const addMinutes = (time: string, minutes: number): string => {
  const [hours = 0, mins = 0] = time.split(":").map(Number);
  const total = (((hours * 60 + mins + minutes) % 1440) + 1440) % 1440;
  return `${String(Math.floor(total / 60)).padStart(2, "0")}:${String(total % 60).padStart(2, "0")}`;
};

/**
 * StartWindow formats when a workout can start, e.g. "07:00–08:00", or ""
 * for any time of the day.
 */
export const startWindow = (workout: Workout): string => {
  if (workout.startTime === "") {
    return "";
  }
  const minutes =
    workout.startWindowMinutes === 0
      ? DEFAULT_START_WINDOW_MINUTES
      : workout.startWindowMinutes;
  return `${workout.startTime}–${addMinutes(workout.startTime, minutes)}`;
};
