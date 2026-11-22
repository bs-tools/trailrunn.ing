import Card from "@components/ui/card";
import { linkClass } from "@components/ui/styles";
import { ExclamationTriangleIcon } from "@heroicons/react/24/outline";
import type { GearAdvice, GearItem } from "gen/plans/v1/gear_pb";
import type { FuelAmounts } from "gen/plans/v1/plans_pb";
import React from "react";
import { Link } from "react-router";

import { gearLabel } from "./gear-labels";

// FormatFluid formats milliliters, in liters from a liter up.
const formatFluid = (ml: number): string =>
  ml >= 1000 ? `${(ml / 1000).toFixed(1)} L` : `${Math.round(ml)} ml`;

const Item: React.FC<{ item: GearItem }> = ({ item }) => (
  <li className="flex items-baseline justify-between gap-3 py-1 text-sm">
    <span className="flex items-center gap-1.5 font-medium text-gray-900">
      {gearLabel(item.gear)}
      {item.missing && (
        <span
          className="inline-flex items-center gap-0.5 text-xs font-normal text-amber-700"
          title="Not in your gear"
        >
          <ExclamationTriangleIcon className="size-3.5" />
          not in your gear
        </span>
      )}
    </span>
    <span className="text-right text-xs text-gray-500">{item.reason}</span>
  </li>
);

interface GearAdviceCardProps {
  advice: GearAdvice;
  // The fueling plan's amounts, carried with the gear, if the run has one.
  fueling: FuelAmounts | undefined;
}

/**
 * GearAdviceCard shows what to wear and carry on a workout, from its
 * forecast and the user's gear, with the fluid and food of its fueling
 * plan.
 */
const GearAdviceCard: React.FC<GearAdviceCardProps> = ({ advice, fueling }) => {
  const supplies: string[] = [];
  if (fueling !== undefined && fueling.fluidMl > 0) {
    supplies.push(`${formatFluid(fueling.fluidMl)} fluid`);
  }
  if (fueling !== undefined && fueling.carbsGrams > 0) {
    supplies.push(`${Math.round(fueling.carbsGrams)} g carbs`);
  }
  if (fueling !== undefined && fueling.sodiumMg > 0) {
    supplies.push(`${Math.round(fueling.sodiumMg)} mg sodium`);
  }
  const anyMissing = [...advice.wear, ...advice.carry].some(
    (item) => item.missing
  );
  return (
    <Card title="What to wear and carry">
      <div className="flex flex-col gap-3">
        <p className="text-sm text-gray-600">{advice.basis}.</p>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <section>
            <h3 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
              Wear
            </h3>
            <ul className="divide-y divide-gray-100">
              {advice.wear.map((item) => (
                <Item key={item.gear} item={item} />
              ))}
            </ul>
          </section>
          <section>
            <h3 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
              Carry
            </h3>
            {advice.carry.length === 0 && supplies.length === 0 ? (
              <p className="py-1 text-sm text-gray-500">Nothing extra.</p>
            ) : (
              <ul className="divide-y divide-gray-100">
                {supplies.length > 0 && (
                  <li className="flex items-baseline justify-between gap-3 py-1 text-sm">
                    <span className="font-medium text-gray-900 tabular-nums">
                      {supplies.join(" · ")}
                    </span>
                    <span className="text-right text-xs text-gray-500">
                      Fueling plan
                    </span>
                  </li>
                )}
                {advice.carry.map((item) => (
                  <Item key={item.gear} item={item} />
                ))}
              </ul>
            )}
          </section>
        </div>
        <p className="text-xs text-gray-500">
          {anyMissing
            ? "Gear marked not in your gear is what the conditions call for. "
            : ""}
          Tell it what you have on the{" "}
          <Link to="/equipment" className={linkClass}>
            equipment page
          </Link>
          .
        </p>
      </div>
    </Card>
  );
};

export default GearAdviceCard;
