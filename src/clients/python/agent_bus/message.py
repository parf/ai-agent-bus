"""Immutable received envelopes; response helpers preserve daemon routing."""
from dataclasses import dataclass, field
from types import MappingProxyType
import threading


def freeze(value):
    if isinstance(value, dict):
        return MappingProxyType({k: freeze(v) for k, v in value.items()})
    if isinstance(value, list):
        return tuple(map(freeze, value))
    return value


class Completion:
    def __init__(self):
        self.lock = threading.Lock()
        self.terminal = False


@dataclass(frozen=True)
class Message:
    envelope: object
    _bus: object = field(repr=False, compare=False)
    _completion: Completion = field(default_factory=Completion, repr=False, compare=False)

    @classmethod
    def received(cls, data, bus):
        return cls(freeze(data), bus)

    def __getattr__(self, name):
        key = {"id": "message_id", "original_to": "original_to"}.get(name, name)
        if key == "roles":
            return self.envelope.get(key, ())
        if key in self.envelope:
            return self.envelope[key]
        if key in ("topic", "tag", "receipt", "re", "ask", "reply_to", "deadline", "expires", "original_to"):
            return None
        raise AttributeError(name)

    def reply(self, body):
        return self._bus.reply(self, body)

    def ack(self):
        return self._bus.ack(self)

    def done(self):
        return self._bus.done(self)


@dataclass(frozen=True)
class CallReply:
    message: Message
    receipts: tuple = ()


@dataclass(frozen=True)
class CallDone:
    message: Message
    receipts: tuple = ()


class Secret:
    __slots__ = ("__bytes",)

    def __init__(self, value):
        self.__bytes = bytes(value)

    def reveal(self):
        return self.__bytes

    def __repr__(self):
        return "Secret[redacted]"

    __str__ = __repr__
