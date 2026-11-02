package agentbus;

import agentbus.Options.Call;
import agentbus.Options.Connect;

import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * An agent writes to its own private event log, its Owner reads and lists it,
 * and nobody else may. Every call goes to the daemon's own #bus agent. BusTest
 * runs this against the fixture and compares the printed lines; it is never
 * pointed at a live bus.
 */
final class EventLogExample {
    private EventLogExample() {}

    /** The lines the example prints, for the fixture's address and tokens. */
    static List<String> run(String addr, Map<?, ?> tokens) {
        Bus agent = dial(addr, tokens, "#caller@fixture"), owner = dial(addr, tokens, "owner@fixture"), stranger = dial(addr, tokens, "#outsider@fixture");
        List<String> lines = new ArrayList<>();
        Map<?, ?> entry = (Map<?, ?>) call(agent, "write #caller@fixture alert disk full").get("entry");
        lines.add("written: " + entry.get("level") + " " + entry.get("text"));
        List<?> alerts = (List<?>) ((Map<?, ?>) call(owner, "read #caller@fixture").get("rings")).get("alert");
        lines.add("owner reads: " + alerts.size() + " alert, newest " + ((Map<?, ?>) alerts.get(0)).get("text"));
        lines.add("owner lists: " + ((Map<?, ?>) ((List<?>) call(owner, "list").get("logs")).get(0)).get("log"));
        lines.add("a third party: " + ((Map<?, ?>) call(stranger, "read #caller@fixture").get("error")).get("code"));
        lines.add("the writer reading its own log: " + ((Map<?, ?>) call(agent, "read #caller@fixture").get("error")).get("code"));
        return lines;
    }

    private static Bus dial(String addr, Map<?, ?> tokens, String name) {
        return Bus.connect(Connect.of(addr, (String) tokens.get(name)).expectName(name).timeout(Duration.ofSeconds(10)));
    }

    // call waits for the answer; a send without wait is accepted and never
    // answered, so it proves only that the message was queued.
    private static Map<?, ?> call(Bus bus, String body) {
        if (!(bus.call("#bus", body, Call.within(Duration.ofSeconds(5))) instanceof Outcome.Answer answer)) {
            throw new IllegalStateException("no answer to " + body);
        }
        return (Map<?, ?>) Json.parse(answer.message().body());
    }
}
