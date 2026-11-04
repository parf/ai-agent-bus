"""Wire compatibility is independent of the shared program version."""
from .errors import BusError

PROTOCOL = 1

def check_identity(identity):
    if not isinstance(identity, dict):
        raise BusError('protocol error: identity must be an object')
    values = []
    for field in ('protocol', 'protocol_min'):
        value = identity.get(field)
        if type(value) is not int or value < 0:
            raise BusError('protocol error: ' + field + ' must be a nonnegative integer')
        values.append(value)
    protocol, minimum = values
    if minimum > protocol:
        raise BusError('protocol error: protocol_min exceeds protocol')
    if PROTOCOL < minimum:
        raise BusError('daemon too new for this client')
    if protocol != PROTOCOL or minimum != PROTOCOL:
        raise BusError('daemon protocol is unsupported')
