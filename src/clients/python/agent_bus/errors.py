"""Local failures and unmodified daemon refusals."""

class BusError(Exception):
    pass

class Refusal(BusError):
    def __init__(self, status, message):
        super().__init__(message)
        self.status = status

class TransportError(BusError):
    pass

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
