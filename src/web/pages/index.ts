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
route("GET", "/resources", rec.resources);
route("GET", "/agents/new", rec.newAgent);
route("GET", "/services/new", rec.newService);
route("GET", "/queues/new", rec.newQueue);
route("GET", "/pubsub/new", rec.newPubsub);
route("GET", "/resources/new", rec.newResource);
route("GET", "/agent", rec.agent);
route("GET", "/service", rec.service);
route("GET", "/queue", rec.queue);
route("GET", "/pubsub/channel", rec.pubsubChannel);
route("GET", "/resource", rec.resource);
for (const p of ["/agent/edit", "/service/edit", "/queue/edit", "/pubsub/channel/edit", "/resource/edit"]) route("GET", p, rec.edit);
route("GET", "/record-deactivate", rec.deactivate);
route("GET", "/record-danger", rec.danger);
route("POST", "/record", rec.post);
route("POST", "/record-confirm", rec.confirm);
// Their names before 0.8.108, kept one release: a page moved, a form still posts.
for (const [from, to] of [["/pubsub/topic", "/pubsub/channel"], ["/pubsub/topic/edit", "/pubsub/channel/edit"], ["/service-deactivate", "/record-deactivate"], ["/service-danger", "/record-danger"]]) route("GET", from, rec.moved(to), false);
route("POST", "/service", rec.post);
route("POST", "/service-confirm", rec.confirm);
route("GET", "/channels", rec.channels, false);
route("GET", "/channels/new", rec.channelsNew, false);
route("GET", "/channel", rec.channel);
route("GET", "/channel/edit", rec.channelEdit);

import { handlers as ppl } from "./people.tsx";
import { handlers as lk } from "./locks.tsx";
import { handlers as kv } from "./kv.tsx";
route("GET", "/users", ppl.users);
route("GET", "/users/new", ppl.newUser);
route("GET", "/user", ppl.user);
route("GET", "/user/edit", ppl.editUser);
route("GET", "/user-danger", ppl.userDanger);
route("GET", "/user-deactivate", ppl.deactivateUser);
route("GET", "/credential-remove", ppl.credentialRemove);
route("POST", "/user", ppl.postUser);
route("GET", "/groups", ppl.groups);
route("GET", "/groups/new", ppl.newGroup);
route("GET", "/group", ppl.group);
route("GET", "/group/edit", ppl.editGroup);
route("POST", "/groups", ppl.postGroups);
route("GET", "/locks", lk.locks);
route("POST", "/release-lock", ppl.postReleaseLock);
route("GET", "/kv", kv.kv);
route("GET", "/kv/record", kv.store);
route("GET", "/kv/value", kv.value);
route("POST", "/kv-set", kv.postSet);
route("POST", "/kv-inc", kv.postInc);
route("POST", "/kv-delete", kv.postDelete);
route("GET", "/account", ppl.account);
route("GET", "/palette.json", ppl.palette, false);

import { handlers as run } from "./runners.tsx";
route("GET", "/runners", run.runners);
route("GET", "/runner", run.runner);
route("POST", "/runner-reload", run.postReload);
route("POST", "/runner-control", run.postControl);
route("GET", "/runner-settings", run.settings);
route("POST", "/runner-file", run.postFile);
route("POST", "/runner-instance", run.postInstance);
route("GET", "/runner-instance-delete", run.instanceDelete);

import { styleguide } from "./styleguide.tsx";
route("GET", "/_styleguide", styleguide, false);

import { agentStatus } from "../agent-status.tsx";
route("GET", "/agent-status", agentStatus);
