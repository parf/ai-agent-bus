/** Configuration and KV dictionaries contain data, never inherited properties. */
export function own<T>(map: Record<string, T> | undefined, key: string): T | undefined {
  return map && Object.hasOwn(map, key) ? map[key] : undefined;
}
