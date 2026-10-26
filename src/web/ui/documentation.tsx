// Fixed public destinations only: record names, filters and credentials never
// become part of a documentation URL.
import {h} from "../jsx.ts";
import {DocumentationLink} from "./kit.tsx";

type Doc = {label:string; target:string};
const record:Doc={label:"Record fields",target:"03-records.md#fields"};
const tags:Doc={label:"Tags",target:"03-records.md#tags"};
const access:Doc={label:"Access rules",target:"02-access.md#acl"};
const configuration:Doc={label:"Configuration",target:"03-records-agent.md#configuring-a-template"};
const secrets:Doc={label:"Secrets (ENV)",target:"03-records-service.md#secrets"};
const overview:Doc={label:"AgentBus guide",target:"00-overview.md#goal"};

export const PAGE_DOCUMENTATION:Record<string,Doc[]>={
 overview:[overview,{label:"Dashboard",target:"05-discovery.md#dashboard"}],
 agents:[{label:"Agents",target:"03-records-agent.md#what-an-agent-is"},record,tags],
 services:[{label:"Services",target:"03-records-service.md#what-a-service-is"},record,tags],
 queues:[{label:"Queues",target:"03-records-channel.md#the-two-channel-kinds"},{label:"Queue policy",target:"04-messaging.md#overflow"},tags],
 pubsub:[{label:"PubSub",target:"03-records-channel.md#the-two-channel-kinds"},{label:"Deliver-To",target:"04-messaging.md#deliver-to"},tags],
 resources:[{label:"Resources",target:"03-records-resource.md#what-a-resource-is"},record,tags],
 groups:[{label:"Groups",target:"01-identity-and-authority.md#groups"},access,tags],
 "kind:group":[{label:"Users",target:"01-identity-and-authority.md#users-and-profiles"},{label:"User states",target:"01-identity-and-authority.md#user-states"}],
 account:[{label:"Your profile",target:"01-identity-and-authority.md#users-and-profiles"},{label:"Credentials",target:"02-access.md#getting-a-token"}],
 runners:[{label:"Runners",target:"runner/operations.md#reading-status"},{label:"Runner access",target:"runner/security.md#who-may-manage-the-runner"}],
 locks:[{label:"Locks",target:"01-identity-and-authority.md#shared-locks"}],
 kv:[{label:"Key-value stores",target:"03-records-kv.md#rules"},{label:"JSON operations",target:"03-records-kv.md#json-operations"}],
 activity:[{label:"Activity history",target:"05-discovery.md#activity-history"},{label:"Message fields",target:"04-messaging.md#message-fields"}],
 diagnostics:[{label:"Diagnostics",target:"05-discovery.md#overview-and-diagnostics"},{label:"Logs",target:"07-setup.md#logs"}],
};

export function pageDocumentation(section:string,path:string,method:string,signedIn:boolean):Doc[]{
 if(!signedIn)return [overview,{label:"Signing in",target:"02-access.md#getting-a-token"}];
 if(section==="kind:group"&&/danger|deactivate|confirm/.test(path))return [PAGE_DOCUMENTATION["kind:group"][1],{label:"Credentials",target:"02-access.md#token-lifetime"}];
 if(/danger|deactivate|confirm/.test(path))return [record,{label:"Ownership",target:"01-identity-and-authority.md#ownership"},{label:"Deletion",target:"01-identity-and-authority.md#unregistering"}];
 if(path==="/credential-remove")return [{label:"Credentials",target:"02-access.md#token-lifetime"}];
 const docs=[...(PAGE_DOCUMENTATION[section]??[overview,access])];
 const editor=path.endsWith("/edit")||path.endsWith("/new")||method==="POST";
 if(editor&&["agents","services","queues","pubsub","resources","groups"].includes(section)){
  if(!docs.some(d=>d.target===record.target))docs.push(record);
  if(!docs.some(d=>d.target===access.target))docs.push(access);
 }
 return docs;
}

export const DocumentationLinks=({section,path,method,signedIn}:{section:string;path:string;method:string;signedIn:boolean})=><nav class="documentation-links" aria-label="Documentation"><span class="documentation-label">Docs</span>{pageDocumentation(section,path,method,signedIn).map(d=><DocumentationLink {...d}/>)}</nav>;

// Field guidance augments the existing help popover, not another row of links.
export const FIELD_DOCUMENTATION:Record<string,Doc & {tip:string}>={
 name:{label:"Naming rules",target:"01-identity-and-authority.md#names",tip:"Names identify records and cannot be changed after registration."},
 tags:{...tags,tip:"Comma-separated labels; letters, digits, hyphens and underscores only; no spaces. Stored lowercase. The personal tag retains its existing Owner and audience rules."},
 personal:{label:"Personal records",target:"03-records.md#personal-and-shared",tip:"Personal classification retains the Owner's audience and management restrictions."},
 allow:{...access,tip:"The allow list decides who may see and use this record."},
 maintainers:{label:"Record authority",target:"01-identity-and-authority.md#record-authority",tip:"Maintainers manage the record; its Owner or the daemon Owner assigns management positions."},
 owner:{label:"Ownership",target:"01-identity-and-authority.md#ownership",tip:"The owning User holds authority over this record."},
 roles:{label:"Roles",target:"03-records-agent.md#roles",tip:"Declare roles the agent understands; assign caller roles through its allow list."},
 subs:{label:"Deliver-To",target:"04-messaging.md#deliver-to",tip:"A destination receives messages forwarded or copied from this record."},
 ttl:{label:"Message retention",target:"04-messaging.md#message-ttl",tip:"How long an unread message may remain in the inbox."},
 bound:{label:"Queue policy",target:"04-messaging.md#overflow",tip:"Maximum queued messages; zero selects the default."},
 overflow:{label:"Queue policy",target:"04-messaging.md#overflow",tip:"Choose whether a full inbox refuses new work or drops its oldest message."},
 addr:{label:"Service addresses",target:"03-records-service.md#how-to-call-it",tip:"Where a caller reaches this external service."},
 protocol:{label:"Service protocols",target:"03-records-service.md#how-to-call-it",tip:"A protocol hint for callers reaching the service at its address."},
 secret:{...secrets,tip:"Keep secrets in Secret and config in Configuration. Dotenv assignments, checked before saving. Comments, optional export and quoted multiline values are allowed; nothing is evaluated."},
 config:{...configuration,tip:"Keep secrets in Secret and config in Configuration. JSON settings visible and editable to its Owner and Maintainers."},
 members:{label:"Group membership",target:"01-identity-and-authority.md#groups",tip:"Users, agents and nested groups in this group's membership."},
};
for(const name of ["uri","template","source","mime","title","mcp_name","icons","annotations"])FIELD_DOCUMENTATION[name]={label:"Resource cards",target:"03-records-resource.md#what-a-resource-is",tip:"Resource metadata describes data the face reads from its source."};
