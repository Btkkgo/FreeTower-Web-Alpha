import {
  definitions,
  nodes,
  type ResourceHit,
  type ResourceType,
} from "./model";
export type GatherSoundSink = (type: ResourceType, depleted: boolean) => void;
// Integration point for later approved audio assets. Current default is deliberately silent.
let sink: GatherSoundSink = () => {};
export function setGatherSoundSink(next: GatherSoundSink) {
  sink = next;
}
export function playGatherHit(hit: ResourceHit) {
  const node = nodes.find((n) => n.id === hit.resourceId);
  if (node) sink(definitions[node.definitionId].type, hit.depleted);
}
