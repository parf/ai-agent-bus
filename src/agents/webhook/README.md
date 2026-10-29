# Webhook bridge

📌 **TL;DR:** A kept stdlib listener forwards authenticated UTF-8 POST bodies to
fixed queues/agents; `post` calls a named configured outbound URL once. The
[guide](../../../docs/agents/webhook.md#what-it-does) owns the root route-list
Configuration, Secret references, bounds, exposure and delivery limits.
