import type { Point } from "./model";

/** Match an authoritative position to the prediction that produced that move,
 * rather than to the player's newer position when its delayed echo arrives. */
export class MovementReconciler {
  private sequence = 0;
  private acknowledged = 0;
  private pending = new Map<number, Point>();

  packet<T extends { position: Point }>(payload: T) {
    const moveSequence = ++this.sequence;
    const position = { ...payload.position };
    this.pending.set(moveSequence, { ...position });
    if (this.pending.size > 256) {
      this.pending.delete(this.pending.keys().next().value!);
    }
    return { ...payload, position, moveSequence };
  }

  reconcile(authoritative: Point, acknowledged: number | undefined, predicted: Point): Point {
    // Keep older, sequence-less servers compatible during a client/server upgrade.
    if (acknowledged === undefined) {
      return Math.hypot(authoritative.x - predicted.x, authoritative.z - predicted.z) > 2
        ? { ...authoritative }
        : predicted;
    }
    if (!Number.isSafeInteger(acknowledged) || acknowledged <= this.acknowledged) return predicted;
    const sent = this.pending.get(acknowledged);
    if (!sent) return predicted;
    const dx = authoritative.x - sent.x;
    const dz = authoritative.z - sent.z;
    this.acknowledged = acknowledged;
    for (const [sequence, position] of this.pending) {
      if (sequence <= acknowledged) this.pending.delete(sequence);
      else this.pending.set(sequence, { x: position.x + dx, z: position.z + dz });
    }
    return dx === 0 && dz === 0 ? predicted : { x: predicted.x + dx, z: predicted.z + dz };
  }
}
