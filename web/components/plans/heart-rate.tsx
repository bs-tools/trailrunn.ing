import { create } from "@bufbuild/protobuf";
import { inputClass, labelClass } from "@components/ui/styles";
import type { HeartRateTarget } from "gen/plans/v1/plans_pb";
import {
  HeartRateReference,
  HeartRateTargetSchema,
} from "gen/plans/v1/plans_pb";
import React from "react";

// The heart rate zones a range can be of, in Uphill Athlete's system.
const ZONES = [1, 2, 3, 4];

/**
 * HeartRateChoice is what a heart rate editor holds: the default, a range
 * of zones, an exact range as typed, or a range relative to a threshold,
 * which agents set and the editor keeps as it is.
 */
export type HeartRateChoice =
  | { mode: "default" }
  | { mode: "zones"; lowZone: number; highZone: number }
  | { mode: "exact"; lowBpm: string; highBpm: string }
  | { mode: "threshold"; target: HeartRateTarget };

// ChoiceOf returns the choice a stored heart rate target is.
export const choiceOf = (
  target: HeartRateTarget | undefined
): HeartRateChoice => {
  if (target === undefined) {
    return { mode: "default" };
  }
  if (target.lowZone > 0) {
    return {
      highZone: target.highZone,
      lowZone: target.lowZone,
      mode: "zones",
    };
  }
  if (target.reference !== HeartRateReference.UNSPECIFIED) {
    return { mode: "threshold", target };
  }
  return {
    highBpm: String(target.highBpm),
    lowBpm: String(target.lowBpm),
    mode: "exact",
  };
};

/**
 * TargetOf returns the heart rate target of a choice, undefined for the
 * default, or an error to show if the choice isn't a range yet.
 */
export const targetOf = (
  choice: HeartRateChoice
): { target: HeartRateTarget | undefined } | { error: string } => {
  switch (choice.mode) {
    case "default": {
      return { target: undefined };
    }
    case "threshold": {
      return { target: choice.target };
    }
    case "zones": {
      return {
        target: create(HeartRateTargetSchema, {
          highZone: Math.max(choice.lowZone, choice.highZone),
          lowZone: choice.lowZone,
        }),
      };
    }
    default: {
      const low = Number(choice.lowBpm);
      const high = Number(choice.highBpm);
      if (
        !Number.isInteger(low) ||
        !Number.isInteger(high) ||
        low < 40 ||
        high > 230 ||
        high < low
      ) {
        return {
          error:
            "The heart rate range is from 40 to 230 bpm, the lower heart rate first.",
        };
      }
      return {
        target: create(HeartRateTargetSchema, { highBpm: high, lowBpm: low }),
      };
    }
  }
};

// Offset formats an offset in bpm from a threshold, e.g. "−5" or "+3".
const offset = (bpm: number): string => {
  if (bpm === 0) {
    return "";
  }
  return bpm > 0 ? ` +${bpm}` : ` −${Math.abs(bpm)}`;
};

/**
 * FormatHeartRate formats a heart rate target, e.g. "Zones 1–2",
 * "125–140 bpm" or "AeT −15 to AeT bpm".
 */
export const formatHeartRate = (target: HeartRateTarget): string => {
  if (target.lowZone > 0) {
    return target.highZone > target.lowZone
      ? `Zones ${target.lowZone}–${target.highZone}`
      : `Zone ${target.lowZone}`;
  }
  let name = "";
  if (target.reference === HeartRateReference.AEROBIC_THRESHOLD) {
    name = "AeT";
  } else if (target.reference === HeartRateReference.ANAEROBIC_THRESHOLD) {
    name = "AnT";
  }
  if (name === "") {
    return `${target.lowBpm}–${target.highBpm} bpm`;
  }
  return `${name}${offset(target.lowBpm)} to ${name}${offset(target.highBpm)} bpm`;
};

interface HeartRateFieldProps {
  label: string;
  // What the default is, e.g. "Default: zones 1–2".
  defaultLabel: string;
  value: HeartRateChoice;
  onChange: (value: HeartRateChoice) => void;
}

/**
 * HeartRateField chooses a heart rate range: the default, a range of zones,
 * or exact heart rates.
 */
export const HeartRateField: React.FC<HeartRateFieldProps> = ({
  label,
  defaultLabel,
  value,
  onChange,
}) => (
  <div className="flex flex-col gap-1">
    <span className={labelClass}>{label}</span>
    <div className="flex flex-wrap items-center gap-2">
      <select
        aria-label={label}
        className={`${inputClass} w-auto!`}
        value={value.mode}
        onChange={(event) => {
          switch (event.target.value) {
            case "zones": {
              onChange({ highZone: 2, lowZone: 1, mode: "zones" });
              break;
            }
            case "exact": {
              onChange({ highBpm: "", lowBpm: "", mode: "exact" });
              break;
            }
            case "default": {
              onChange({ mode: "default" });
              break;
            }
            default: {
              // The range relative to a threshold stays selected.
              break;
            }
          }
        }}
      >
        <option value="default">{defaultLabel}</option>
        <option value="zones">Zones</option>
        <option value="exact">Exact range</option>
        {value.mode === "threshold" && (
          <option value="threshold">{formatHeartRate(value.target)}</option>
        )}
      </select>
      {value.mode === "zones" && (
        <>
          <select
            aria-label="From zone"
            className={`${inputClass} w-auto!`}
            value={value.lowZone}
            onChange={(event) => {
              const lowZone = Number(event.target.value);
              onChange({
                highZone: Math.max(lowZone, value.highZone),
                lowZone,
                mode: "zones",
              });
            }}
          >
            {ZONES.map((zone) => (
              <option key={zone} value={zone}>
                Zone {zone}
              </option>
            ))}
          </select>
          <span className="text-sm text-gray-600">to</span>
          <select
            aria-label="To zone"
            className={`${inputClass} w-auto!`}
            value={value.highZone}
            onChange={(event) => {
              onChange({ ...value, highZone: Number(event.target.value) });
            }}
          >
            {ZONES.filter((zone) => zone >= value.lowZone).map((zone) => (
              <option key={zone} value={zone}>
                Zone {zone}
              </option>
            ))}
          </select>
        </>
      )}
      {value.mode === "exact" && (
        <>
          <input
            aria-label="Lowest heart rate (bpm)"
            className={`${inputClass} w-20!`}
            inputMode="numeric"
            placeholder="120"
            value={value.lowBpm}
            onChange={(event) => {
              onChange({ ...value, lowBpm: event.target.value });
            }}
          />
          <span className="text-sm text-gray-600">to</span>
          <input
            aria-label="Highest heart rate (bpm)"
            className={`${inputClass} w-20!`}
            inputMode="numeric"
            placeholder="140"
            value={value.highBpm}
            onChange={(event) => {
              onChange({ ...value, highBpm: event.target.value });
            }}
          />
          <span className="text-sm text-gray-600">bpm</span>
        </>
      )}
    </div>
  </div>
);
