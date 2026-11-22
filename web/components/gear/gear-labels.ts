import { Gear } from "gen/plans/v1/gear_pb";

// GearGroup is a group of gear, as the equipment page lists it.
export interface GearGroup {
  title: string;
  gear: Gear[];
}

// GEAR_GROUPS lists all the gear, grouped as it's worn.
export const GEAR_GROUPS: GearGroup[] = [
  {
    gear: [Gear.SHORTS, Gear.TIGHTS, Gear.THERMAL_TIGHTS, Gear.RAIN_PANTS],
    title: "Legs",
  },
  {
    gear: [
      Gear.T_SHIRT,
      Gear.LONG_SLEEVE,
      Gear.BASE_LAYER,
      Gear.FLEECE,
      Gear.WIND_JACKET,
      Gear.RAIN_JACKET,
      Gear.INSULATED_JACKET,
    ],
    title: "Tops and jackets",
  },
  {
    gear: [
      Gear.CAP,
      Gear.TOQUE,
      Gear.BUFF,
      Gear.SUNGLASSES,
      Gear.LINER_GLOVES,
      Gear.GLOVES,
      Gear.THICK_GLOVES,
    ],
    title: "Head and hands",
  },
  {
    gear: [
      Gear.ROAD_SHOES,
      Gear.TRAIL_SHOES,
      Gear.WATERPROOF_SHOES,
      Gear.MICROSPIKES,
    ],
    title: "Feet",
  },
  {
    gear: [Gear.RUNNING_VEST, Gear.RUNNING_BELT, Gear.HEADLAMP, Gear.POLES],
    title: "To carry",
  },
  {
    gear: [
      Gear.SOFTSHELL_PANTS,
      Gear.SHELL_PANTS,
      Gear.SHELL_JACKET,
      Gear.SKI_GLOVES,
      Gear.GOGGLES,
      Gear.HELMET,
      Gear.AVALANCHE_TRANSCEIVER,
      Gear.SHOVEL,
      Gear.PROBE,
      Gear.SKI_CRAMPONS,
    ],
    title: "Ski touring",
  },
];

const GEAR_LABELS: Record<Gear, string> = {
  [Gear.UNSPECIFIED]: "Gear",
  [Gear.SHORTS]: "Shorts",
  [Gear.TIGHTS]: "Tights",
  [Gear.THERMAL_TIGHTS]: "Thermal tights",
  [Gear.RAIN_PANTS]: "Rain pants",
  [Gear.T_SHIRT]: "T-shirt",
  [Gear.LONG_SLEEVE]: "Long sleeve",
  [Gear.BASE_LAYER]: "Base layer",
  [Gear.FLEECE]: "Fleece",
  [Gear.WIND_JACKET]: "Wind jacket",
  [Gear.RAIN_JACKET]: "Rain jacket",
  [Gear.INSULATED_JACKET]: "Insulated jacket",
  [Gear.CAP]: "Cap",
  [Gear.TOQUE]: "Toque",
  [Gear.BUFF]: "Buff",
  [Gear.SUNGLASSES]: "Sunglasses",
  [Gear.LINER_GLOVES]: "Liner gloves",
  [Gear.GLOVES]: "Gloves",
  [Gear.THICK_GLOVES]: "Thick gloves",
  [Gear.ROAD_SHOES]: "Road shoes",
  [Gear.TRAIL_SHOES]: "Trail shoes",
  [Gear.WATERPROOF_SHOES]: "Waterproof shoes",
  [Gear.MICROSPIKES]: "Microspikes",
  [Gear.RUNNING_VEST]: "Running vest",
  [Gear.RUNNING_BELT]: "Running belt",
  [Gear.HEADLAMP]: "Headlamp",
  [Gear.POLES]: "Poles",
  [Gear.SOFTSHELL_PANTS]: "Softshell pants",
  [Gear.SHELL_PANTS]: "Shell pants",
  [Gear.SHELL_JACKET]: "Shell jacket",
  [Gear.SKI_GLOVES]: "Ski gloves",
  [Gear.GOGGLES]: "Goggles",
  [Gear.HELMET]: "Helmet",
  [Gear.AVALANCHE_TRANSCEIVER]: "Avalanche transceiver",
  [Gear.SHOVEL]: "Shovel",
  [Gear.PROBE]: "Probe",
  [Gear.SKI_CRAMPONS]: "Ski crampons",
};

// GearLabel names a piece of gear, e.g. "Rain jacket".
export const gearLabel = (gear: Gear): string => GEAR_LABELS[gear];
