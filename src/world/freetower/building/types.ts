import raw from "../../../../server/freetower_buildings.json" with { type: "json" };
import type { Point } from "../model.ts";
export interface Footprint {
  width: number;
  depth: number;
}
export interface BuildingDefinition {
  id: string;
  assetId: string;
  displayName: string;
  footprint: Footprint;
  cost: { itemId: string; quantity: number }[];
  constructionSeconds: number;
  rotationStep: number;
  placementRules: {
    maxSlope: number;
    maxHeightDelta: number;
    minElevation: number;
    riverBuffer: number;
    coastBuffer: number;
  };
}
export interface BuildingInstance {
  id: string;
  definitionId: string;
  position: Point & { y: number };
  rotationY: number;
  status: "constructing" | "completed" | "moving";
  constructionStartedAt: number;
  constructionEndsAt: number;
  ownerId: string;
  moveOwnerId?: string;
  moveExpiresAt?: number;
}
export type BuildingMessage =
  | {
      type: "place-building";
      requestId: string;
      definitionId: string;
      position: Point;
      rotationY: number;
    }
  | { type: "begin-building-move"; requestId: string; buildingId: string }
  | {
      type: "move-building";
      requestId: string;
      buildingId: string;
      moveToken: string;
      position: Point;
      rotationY: number;
    }
  | {
      type: "cancel-building-move" | "renew-building-move";
      buildingId: string;
      moveToken: string;
    };
export type BuildingReply =
  | {
      type: "building-result";
      requestId: string;
      buildingId: string;
      moveToken?: string;
    }
  | {
      type: "building-error";
      requestId?: string;
      code: string;
      message: string;
    };
export const buildingConfig = raw;
export const buildingDefinitions: Record<string, BuildingDefinition> =
  Object.fromEntries(raw.definitions.map((d) => [d.id, d]));
