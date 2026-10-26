/** Size the actual textarea value; HTML's synthetic leading newline is not part of it. */
export function rowsFor(value: string, focused: boolean): number {
  const lines = value.split("\n").length;
  return Math.min(20, Math.max(focused ? 5 : 1, lines));
}
