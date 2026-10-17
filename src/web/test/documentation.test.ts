import {expect,test} from "bun:test";
import {readFileSync} from "node:fs";
import {join} from "node:path";
import {render,h} from "../jsx.ts";
import {DocumentationLink,Help} from "../ui/kit.tsx";
import {FIELD_DOCUMENTATION,PAGE_DOCUMENTATION,pageDocumentation} from "../ui/documentation.tsx";

const cases:[string,string,string][]=[
 ["agents","/agent/edit","03-records-agent.md#what-an-agent-is"],
 ["agents","/agents/new","03-records.md#fields"],
 ["services","/services/new","03-records-service.md#what-a-service-is"],
 ["queues","/queue/edit","04-messaging.md#overflow"],
 ["pubsub","/pubsub/channel/edit","04-messaging.md#deliver-to"],
 ["resources","/resources/new","03-records-resource.md#what-a-resource-is"],
 ["groups","/group/edit","01-identity-and-authority.md#groups"],
 ["kind:group","/users/new","01-identity-and-authority.md#users-and-profiles"],
 ["kind:group","/user-deactivate","01-identity-and-authority.md#user-states"],
 ["kind:group","/credential-remove","02-access.md#token-lifetime"],
 ["agents","/record-danger","01-identity-and-authority.md#ownership"],
 ["locks","/locks","01-identity-and-authority.md#shared-locks"],
 ["kv","/kv/value","03-records-kv.md#rules"],
 ["activity","/activity","05-discovery.md#activity-history"],
 ["diagnostics","/diagnostics","07-setup.md#logs"],
 ["account","/account","02-access.md#getting-a-token"],
 ["overview","/","00-overview.md#goal"],
];
test("every page family and its forms link to their own public documentation",()=>{
 for(const [section,path,target] of cases)expect(pageDocumentation(section,path,"GET",true).map(d=>d.target)).toContain(target);
 expect(pageDocumentation("agents","/record","POST",true).map(d=>d.target)).toContain("02-access.md#acl");
 expect(pageDocumentation("agents","/agent/edit","GET",false).map(d=>d.target)).toContain("02-access.md#getting-a-token");
});
test("documentation targets resolve to local canonical files and headings",()=>{
 const docs=[...pageDocumentation("overview","/","GET",false),...pageDocumentation("","/not-found","GET",true),...Object.values(PAGE_DOCUMENTATION).flat(),...cases.flatMap(([s,p])=>pageDocumentation(s,p,"GET",true)),...Object.values(FIELD_DOCUMENTATION)];
 for(const {target} of docs){
  const [file,anchor]=target.split("#");
  const text=readFileSync(join(import.meta.dir,"../../../docs",file!),"utf8");
  const headings=[...text.matchAll(/^#{1,6}\s+(.+)$/gm)].map(m=>m[1]!.toLowerCase().replace(/[`*_]/g,"").replace(/[^\p{L}\p{N}\s_-]/gu,"").trim().replace(/\s/g,"-"));
  expect(`${file}#${anchor}: ${headings.includes(anchor!)}`).toBe(`${file}#${anchor}: true`);
 }
});
test("documentation links are distinct external badges and field help keeps them in its popover",()=>{
 const doc=FIELD_DOCUMENTATION.secret!;
 const link=render(h(DocumentationLink,doc));
 expect(link).toContain('class="docs-link"');
 expect(link).toContain('data-lucide="book-open"');
 expect(link).toContain('data-lucide="external-link"');
 expect(link).toContain('target="_blank" rel="noopener noreferrer"');
 expect(link).toContain('https://github.com/parf/ai-agent-bus/blob/main/docs/03-records-service.md#secrets');
 const help=render(h(Help,{label:"About secret",tip:"Keep it private",doc}));
 expect(help).toMatch(/<div popover[^>]*>[\s\S]*class="docs-link"/);
 expect(help).toContain('data-tooltip="Keep it private"');
});
