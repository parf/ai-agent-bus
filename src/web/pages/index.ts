// Every address the face answers, by page family.
import { route } from "../router.ts";
import { postSignIn, postSignOut } from "./landing.tsx";
import { home, activity, diagnostics } from "./node.tsx";

route("GET", "/", home, false);
route("POST", "/signin", postSignIn, false);
route("POST", "/signout", postSignOut, false);
route("GET", "/activity", activity);
route("GET", "/diagnostics", diagnostics);

import { handlers as rec } from "./records.tsx";
route("GET", "/agents", rec.agents);
route("GET", "/services", rec.services);
route("GET", "/queues", rec.queues);
route("GET", "/pubsub", rec.pubsub);
route("GET", "/agents/new", rec.newAgent);
route("GET", "/services/new", rec.newService);
route("GET", "/queues/new", rec.newQueue);
route("GET", "/pubsub/new", rec.newPubsub);
route("GET", "/agent", rec.agent);
route("GET", "/service", rec.service);
route("GET", "/queue", rec.queue);
route("GET", "/pubsub/topic", rec.topic);
for (const p of ["/agent/edit", "/service/edit", "/queue/edit", "/pubsub/topic/edit"]) route("GET", p, rec.edit);
route("GET", "/service-deactivate", rec.deactivate);
route("GET", "/service-danger", rec.danger);
route("POST", "/service", rec.post);
route("POST", "/service-confirm", rec.confirm);
route("GET", "/channels", rec.channels, false);
route("GET", "/channels/new", rec.channelsNew, false);
route("GET", "/channel", rec.channel);
route("GET", "/channel/edit", rec.channelEdit);

import { handlers as ppl } from "./people.tsx";
route("GET", "/users", ppl.users);
route("GET", "/users/new", ppl.newUser);
route("GET", "/user", ppl.user);
route("GET", "/user/edit", ppl.editUser);
route("GET", "/user-deactivate", ppl.deactivateUser);
route("GET", "/credential-remove", ppl.credentialRemove);
route("POST", "/user", ppl.postUser);
route("GET", "/groups", ppl.groups);
route("GET", "/groups/new", ppl.newGroup);
route("GET", "/group", ppl.group);
route("GET", "/group/edit", ppl.editGroup);
route("POST", "/groups", ppl.postGroups);
route("GET", "/account", ppl.account);
route("GET", "/palette.json", ppl.palette, false);

import { styleguide } from "./styleguide.tsx";
route("GET", "/_styleguide", styleguide, false);
