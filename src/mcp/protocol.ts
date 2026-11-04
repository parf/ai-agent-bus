/** The client wire protocol, independent of the program version. */
export const CLIENT_PROTOCOL = 1;
export function checkProtocol(identity: unknown): void {
  if (!identity || typeof identity !== 'object' || Array.isArray(identity)) throw new Error('protocol error: identity must be an object');
  const row = identity as Record<string,unknown>;
  const fields = ['protocol','protocol_min'] as const;
  const values = fields.map(field => {
    const value = row[field];
    if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) throw new Error(`protocol error: ${field} must be a nonnegative integer`);
    return value;
  });
  const [protocol,minimum] = values as [number,number];
  if (minimum > protocol) throw new Error('protocol error: protocol_min exceeds protocol');
  if (CLIENT_PROTOCOL < minimum) throw new Error('daemon too new for this client');
  if (protocol !== CLIENT_PROTOCOL || minimum !== CLIENT_PROTOCOL) throw new Error('daemon protocol is unsupported');
}
