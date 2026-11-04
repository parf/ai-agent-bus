"""Local failures and unmodified daemon refusals."""

class BusError(Exception):
    pass

class Refusal(BusError):
    def __init__(self, status, message, kind=None):
        super().__init__("internal daemon error" if status == 500 else message)
        self.status = status
        self.kind = kind

class TransportError(BusError):
    pass

class UnexpectedAnswer(BusError):
    """The daemon answered, but not what was asked: a mismatched identity or wrong shape."""

class UncertainOutcome(BusError):
    """The operation may have completed remotely; do not repeat it blindly."""

class BusTimeout(BusError):
    def __init__(self, message="deadline expired", receipts=()):
        super().__init__(message)
        self.receipts = tuple(receipts)

class BusClosed(BusError):
    pass

class BusCancelled(BusError):
    pass

class IdentityMismatch(BusError):
    pass

class ReaderInUse(BusError):
    pass

class InvalidArgument(BusError, ValueError):
    pass

class DrainTimeout(BusTimeout):
    def __init__(self, unfinished):
        super().__init__(f"{unfinished} handlers still running at drain deadline")
        self.unfinished = unfinished
