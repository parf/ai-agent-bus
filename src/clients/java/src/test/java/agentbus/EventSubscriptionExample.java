package agentbus;

import agentbus.Options.Call;
import agentbus.Options.Connect;
import agentbus.Options.Consume;

import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * An Owner copies each alert of an agent's log to a queue, reads the one copy
 * that arrives, and cancels. A copy is a notice from #bus: it is read, never
 * acknowledged and never marked done. BusTest runs this against the fixture and
 * compares the printed lines; it is never pointed at a live bus.
 */
final class EventSubscriptionExample {
    private EventSubscriptionExample() {}

    /** The lines the example prints, for the fixture's address and tokens. */
    static List<String> run(String addr, Map<?, ?> tokens) {
        // Each Bus holds a reader and a connection: close it when done.
        try (Bus agent = dial(addr, tokens, "#caller@fixture");
             Bus owner = dial(addr, tokens, "owner@fixture")) {
            List<String> lines = new ArrayList<>();
            String id = (String) call(owner, "sub add jobs@fixture #caller@fixture alert").get("id");
            lines.add("listed: " + ((List<?>) call(owner, "sub list #caller@fixture").get("subscriptions")).size() + " subscription");
            Map<?, ?> deliveries = (Map<?, ?>) call(agent, "write #caller@fixture alert disk full").get("deliveries");
            lines.add("delivered: " + deliveries.get("sent") + " copy");
            Optional<Message> copy = owner.consume(Consume.waiting(Duration.ofMillis(300)).inbox("jobs@fixture"));
            if (copy.isEmpty()) throw new IllegalStateException("no subscription copy");
            Map<?, ?> entry = (Map<?, ?>) Json.parse(copy.get().body());
            lines.add("copy from " + copy.get().from() + ": " + entry.get("level") + " " + entry.get("text"));
            // A notice: nothing is acknowledged or marked done.
            lines.add("one copy only: " + owner.consume(Consume.waiting(Duration.ofMillis(300)).inbox("jobs@fixture")).isEmpty());
            call(owner, "sub delete " + id);
            Map<?, ?> after = (Map<?, ?>) call(agent, "write #caller@fixture alert after cancellation").get("deliveries");
            lines.add("after cancelling, matched: " + after.get("matched"));
            return lines;
        }
    }

    private static Bus dial(String addr, Map<?, ?> tokens, String name) {
        return Bus.connect(Connect.of(addr, (String) tokens.get(name)).expectName(name).timeout(Duration.ofSeconds(10)));
    }

    private static Map<?, ?> call(Bus bus, String body) {
        if (!(bus.call("#bus", body, Call.within(Duration.ofSeconds(5))) instanceof Outcome.Answer answer)) {
            throw new IllegalStateException("no answer to " + body);
        }
        return (Map<?, ?>) Json.parse(answer.message().body());
    }
}
