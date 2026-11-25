import { clone, create } from "@bufbuild/protobuf";
import { SelectField, TextAreaField, TextField } from "@components/ui/fields";
import type { SelectOption } from "@components/ui/fields";
import { ghostButton, secondaryButton } from "@components/ui/styles";
import {
  ArrowDownIcon,
  ArrowUpIcon,
  PlusIcon,
  TrashIcon,
} from "@heroicons/react/24/outline";
import type { Muscle } from "gen/plans/v1/muscles_pb";
import type {
  ElectricalStimulation,
  RecoveryItem,
} from "gen/plans/v1/plans_pb";
import {
  ElectricalStimulationSchema,
  RecoveryItemSchema,
  RecoveryModality,
  RecoverySide,
  StimulationProgram,
  TensMassage,
} from "gen/plans/v1/plans_pb";
import { parseEnum } from "helpers/parse";
import {
  isTens,
  itemTitle,
  KIND_LABELS,
  MODALITY_LABELS,
  programInfo,
  PROGRAMS,
  TENS_MASSAGE_LABELS,
} from "helpers/recovery";
import { MUSCLE_LABELS } from "helpers/strength";
import React from "react";

const MODALITY_OPTIONS: SelectOption[] = MODALITY_LABELS.map((entry) => ({
  label: entry.label,
  value: String(entry.modality),
}));

// The programs are listed by kind, like the PowerDot app's.
const PROGRAM_OPTIONS: SelectOption[] = [
  { label: "Choose a program", value: String(StimulationProgram.UNSPECIFIED) },
  ...PROGRAMS.map((entry) => ({
    label: `${KIND_LABELS[entry.kind]}: ${entry.label}`,
    value: String(entry.program),
  })),
];

const SIDE_OPTIONS: SelectOption[] = [
  { label: "Both sides", value: String(RecoverySide.UNSPECIFIED) },
  { label: "Left", value: String(RecoverySide.LEFT) },
  { label: "Right", value: String(RecoverySide.RIGHT) },
];

const PODS_OPTIONS: SelectOption[] = [
  { label: "Not set", value: "0" },
  { label: "1 pod", value: "1" },
  { label: "2 pods", value: "2" },
];

const PAIN_OPTIONS: SelectOption[] = [
  { label: "Not set", value: "" },
  ...Array.from({ length: 11 }, (_unused, level) => ({
    label: String(level),
    value: String(level),
  })),
];

const TENS_MASSAGE_OPTIONS: SelectOption[] = [
  { label: "Not set", value: String(TensMassage.UNSPECIFIED) },
  ...TENS_MASSAGE_LABELS.map((entry) => ({
    label: entry.label,
    value: String(entry.massage),
  })),
];

/*
 * NewRecoveryItem returns a new item: electrical stimulation, the reason
 * this kind of workout exists.
 */
export const newRecoveryItem = (): RecoveryItem =>
  create(RecoveryItemSchema, {
    modality: RecoveryModality.ELECTRICAL_STIMULATION,
    stimulation: create(ElectricalStimulationSchema, {}),
  });

// MinutesInput shows seconds as whole minutes, empty for none.
const minutesInput = (seconds: number): string =>
  seconds === 0 ? "" : String(Math.round(seconds / 60));

interface ItemEditorProps {
  item: RecoveryItem;
  index: number;
  count: number;
  onChange: (item: RecoveryItem) => void;
  onMove: (offset: number) => void;
  onRemove: () => void;
}

// ItemEditor edits one recovery item.
const ItemEditor: React.FC<ItemEditorProps> = ({
  item,
  index,
  count,
  onChange,
  onMove,
  onRemove,
}) => {
  const update = (changes: Partial<RecoveryItem>) => {
    onChange(Object.assign(clone(RecoveryItemSchema, item), changes));
  };
  const { stimulation } = item;
  const updateStimulation = (changes: Partial<ElectricalStimulation>) => {
    const next = Object.assign(
      stimulation === undefined
        ? create(ElectricalStimulationSchema, {})
        : clone(ElectricalStimulationSchema, stimulation),
      changes
    );
    // Options only pain programs have are cleared for the others.
    if (!isTens(next.program)) {
      next.painLevel = undefined;
    }
    if (next.program !== StimulationProgram.PAIN_RELIEF) {
      next.tensMassage = TensMassage.UNSPECIFIED;
    }
    update({ stimulation: next });
  };
  const toggleMuscle = (muscle: Muscle) => {
    update({
      muscles: item.muscles.includes(muscle)
        ? item.muscles.filter((other) => other !== muscle)
        : [...item.muscles, muscle],
    });
  };
  const program = stimulation?.program ?? StimulationProgram.UNSPECIFIED;
  return (
    <li className="flex flex-col gap-3 rounded-md border border-gray-200 p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-semibold text-gray-900">
          {index + 1}. {itemTitle(item)}
        </span>
        <span className="flex gap-1">
          <button
            type="button"
            className={ghostButton}
            disabled={index === 0}
            aria-label="Move up"
            onClick={() => {
              onMove(-1);
            }}
          >
            <ArrowUpIcon className="size-4" />
          </button>
          <button
            type="button"
            className={ghostButton}
            disabled={index === count - 1}
            aria-label="Move down"
            onClick={() => {
              onMove(1);
            }}
          >
            <ArrowDownIcon className="size-4" />
          </button>
          <button
            type="button"
            className={ghostButton}
            aria-label="Remove"
            onClick={onRemove}
          >
            <TrashIcon className="size-4" />
          </button>
        </span>
      </div>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        <SelectField
          label="Kind"
          value={String(item.modality)}
          onChange={(value) => {
            const modality = parseEnum(
              RecoveryModality,
              value,
              RecoveryModality.OTHER
            );
            update({
              modality,
              stimulation:
                modality === RecoveryModality.ELECTRICAL_STIMULATION
                  ? (stimulation ?? create(ElectricalStimulationSchema, {}))
                  : undefined,
            });
          }}
          options={MODALITY_OPTIONS}
        />
        <TextField
          label="Minutes"
          value={minutesInput(item.durationSeconds)}
          onChange={(value) => {
            const minutes = Number(value);
            if (value.trim() === "" || Number.isFinite(minutes)) {
              update({
                durationSeconds: Math.max(0, Math.round(minutes * 60)),
              });
            }
          }}
          inputMode="numeric"
        />
        <SelectField
          label="Side"
          value={String(item.side)}
          onChange={(value) => {
            update({
              side: parseEnum(RecoverySide, value, RecoverySide.UNSPECIFIED),
            });
          }}
          options={SIDE_OPTIONS}
        />
      </div>
      {stimulation !== undefined && (
        <div className="flex flex-col gap-3 rounded-md bg-teal-50/60 p-3">
          <SelectField
            label="Program"
            value={String(program)}
            onChange={(value) => {
              updateStimulation({
                program: parseEnum(
                  StimulationProgram,
                  value,
                  StimulationProgram.UNSPECIFIED
                ),
              });
            }}
            options={PROGRAM_OPTIONS}
          />
          {programInfo(program) !== undefined && (
            <p className="-mt-2 text-xs text-gray-600">
              {programInfo(program)?.purpose}
            </p>
          )}
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <SelectField
              label="Pods"
              value={String(stimulation.pods)}
              onChange={(value) => {
                updateStimulation({ pods: Number(value) });
              }}
              options={PODS_OPTIONS}
            />
            <TextField
              label="Intensity (1–100)"
              value={
                stimulation.intensity === 0 ? "" : String(stimulation.intensity)
              }
              onChange={(value) => {
                const intensity = Number(value);
                if (value.trim() === "" || Number.isInteger(intensity)) {
                  updateStimulation({ intensity: Math.max(0, intensity) });
                }
              }}
              inputMode="numeric"
            />
            {isTens(program) && (
              <SelectField
                label="Pain level"
                value={
                  stimulation.painLevel === undefined
                    ? ""
                    : String(stimulation.painLevel)
                }
                onChange={(value) => {
                  updateStimulation({
                    painLevel: value === "" ? undefined : Number(value),
                  });
                }}
                options={PAIN_OPTIONS}
              />
            )}
            {program === StimulationProgram.PAIN_RELIEF && (
              <SelectField
                label="Massage"
                value={String(stimulation.tensMassage)}
                onChange={(value) => {
                  updateStimulation({
                    tensMassage: parseEnum(
                      TensMassage,
                      value,
                      TensMassage.UNSPECIFIED
                    ),
                  });
                }}
                options={TENS_MASSAGE_OPTIONS}
              />
            )}
          </div>
        </div>
      )}
      <fieldset className="flex flex-col gap-1">
        <legend className="text-xs font-medium text-gray-600">
          {stimulation === undefined ? "Muscles" : "Where the pads go"}
        </legend>
        <div className="flex flex-wrap gap-1.5">
          {MUSCLE_LABELS.map((entry) => {
            const on = item.muscles.includes(entry.muscle);
            return (
              <button
                key={entry.muscle}
                type="button"
                aria-pressed={on}
                className={`rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset ${
                  on
                    ? "bg-teal-600 text-white ring-teal-600"
                    : "bg-white text-gray-700 ring-gray-300"
                }`}
                onClick={() => {
                  toggleMuscle(entry.muscle);
                }}
              >
                {entry.label}
              </button>
            );
          })}
        </div>
      </fieldset>
      <TextAreaField
        label="Notes"
        value={item.notes}
        onChange={(notes) => {
          update({ notes });
        }}
        rows={2}
        placeholder={
          stimulation === undefined
            ? "e.g. temperature, pressure level"
            : "e.g. pad placement the physio showed"
        }
      />
    </li>
  );
};

interface RecoveryItemsFieldProps {
  items: RecoveryItem[];
  onChange: (items: RecoveryItem[]) => void;
}

/**
 * RecoveryItemsField edits the items of recovery work, in order: what kind
 * each is, for how long and where, with the program and settings of
 * electrical stimulation.
 */
const RecoveryItemsField: React.FC<RecoveryItemsFieldProps> = ({
  items,
  onChange,
}) => (
  <fieldset className="flex flex-col gap-2">
    <legend className="text-sm font-medium text-gray-700">What to do</legend>
    {items.length > 0 && (
      <ol className="flex flex-col gap-2">
        {items.map((item, index) => (
          <ItemEditor
            // Items have no IDs; their order is their identity here.
            // oxlint-disable-next-line no-array-index-key
            key={index}
            item={item}
            index={index}
            count={items.length}
            onChange={(changed) => {
              onChange(
                items.map((each, at) => (at === index ? changed : each))
              );
            }}
            onMove={(offset) => {
              const moved = [...items];
              const [taken] = moved.splice(index, 1);
              if (taken !== undefined) {
                moved.splice(index + offset, 0, taken);
              }
              onChange(moved);
            }}
            onRemove={() => {
              onChange(items.filter((_unused, at) => at !== index));
            }}
          />
        ))}
      </ol>
    )}
    <div>
      <button
        type="button"
        className={secondaryButton}
        onClick={() => {
          onChange([...items, newRecoveryItem()]);
        }}
      >
        <PlusIcon className="size-4" />
        Add {items.length === 0 ? "an item" : "another"}
      </button>
    </div>
  </fieldset>
);

export default RecoveryItemsField;
