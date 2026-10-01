package agentbus;

import java.time.Instant;
import java.time.OffsetDateTime;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * A received envelope with every field kept (docs/04-messaging.md#message-fields).
 * {@code roles} is the daemon's, never settable by a sender.
 */
public record Message(String id, String from, String to, Optional<String> topic, Optional<String> tag,
                      String body, Instant at, Optional<Receipt> receipt, Optional<String> re,
                      Optional<ReplyTo> replyTo, List<String> roles, Optional<String> originalTo,
                      int forwards, Optional<Instant> expires, Optional<Instant> deadline) {

    public Message {
        roles = List.copyOf(roles);
    }

    public enum Receipt { ACK, DONE }

    /** Where an answer goes, and under which topic and tag. */
    public record ReplyTo(String name, Optional<String> topic, Optional<String> tag) {
        public static ReplyTo of(String name, String topic, String tag) {
            return new ReplyTo(name, Optional.ofNullable(topic), Optional.ofNullable(tag));
        }

        Map<String, Object> toJson() {
            var m = new java.util.LinkedHashMap<String, Object>();
            m.put("name", name);
            topic.ifPresent(t -> m.put("topic", t));
            tag.ifPresent(t -> m.put("tag", t));
            return m;
        }
    }

    /** Where an answer to this goes: the sender, unless it named somewhere else. */
    public ReplyTo route() {
        return replyTo.orElseGet(() -> new ReplyTo(from, topic, tag));
    }

    public boolean isReceipt() { return receipt.isPresent(); }

    /** Its caller has stopped waiting: working on it now helps nobody. */
    public boolean tooLate(Instant now) {
        return deadline.map(now::isAfter).orElse(false);
    }

    @SuppressWarnings("unchecked")
    static Message of(Map<String, Object> e) {
        Object rt = e.get("reply_to");
        Optional<ReplyTo> replyTo = Optional.empty();
        if (rt instanceof Map<?, ?> r) {
            replyTo = Optional.of(new ReplyTo(String.valueOf(r.get("name")), text(r.get("topic")), text(r.get("tag"))));
        }
        Optional<Receipt> receipt = text(e.get("receipt")).map(s -> s.equals("ack") ? Receipt.ACK : Receipt.DONE);
        Object roles = e.get("roles");
        return new Message(
                String.valueOf(e.getOrDefault("message_id", "")),
                String.valueOf(e.getOrDefault("from", "")),
                String.valueOf(e.getOrDefault("to", "")),
                text(e.get("topic")), text(e.get("tag")),
                String.valueOf(e.getOrDefault("body", "")),
                time(e.get("at")).orElse(Instant.EPOCH),
                receipt, text(e.get("re")), replyTo,
                roles instanceof List<?> l ? (List<String>) (List<?>) l.stream().map(String::valueOf).toList() : List.of(),
                text(e.get("original_to")),
                e.get("forwards") instanceof Number n ? n.intValue() : 0,
                time(e.get("expires")), time(e.get("deadline")));
    }

    private static Optional<String> text(Object v) {
        return v instanceof String s && !s.isEmpty() ? Optional.of(s) : Optional.empty();
    }

    private static Optional<Instant> time(Object v) {
        // Go writes an unset moment as the zero time.
        if (!(v instanceof String s) || s.isEmpty() || s.startsWith("0001-01-01")) return Optional.empty();
        return Optional.of(OffsetDateTime.parse(s).toInstant());
    }
}
