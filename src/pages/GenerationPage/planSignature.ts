/**
 * Plan-preview signature bookkeeping.
 *
 * The preflight request is asynchronous: the user may edit the draft while it
 * is in flight. The response must be attributed to the signature that was
 * current when the request STARTED (not when it completed), otherwise an old
 * plan would be treated as fresh and could be confirmed with a mismatched
 * draft.
 */
export function capturePlanSignature(currentSignature: string): string {
  return currentSignature;
}

export function isPlanResponseStale(requestedSignature: string, currentSignature: string): boolean {
  return requestedSignature !== currentSignature;
}
