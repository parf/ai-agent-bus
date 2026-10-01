package agentbus;

/** What a call ended in: an answer, a done receipt, or the caller's deadline. */
public sealed interface Outcome permits Outcome.Answer, Outcome.Done, Outcome.TimedOut {
    /** The answer; {@code acked} says an ack came first. */
    record Answer(Message message, boolean acked) implements Outcome {}

    /** The receiver finished with no answer. */
    record Done(Message receipt) implements Outcome {}

    /** Nothing final came before the deadline. The remote work may still run. */
    record TimedOut(boolean acked) implements Outcome {}
}
