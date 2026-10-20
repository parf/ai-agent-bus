import {expect,test} from "bun:test";
import {parseBusName} from "../shared/names.ts";
import vectors from "../../internal/protocol/testdata/names.json";
import {routingConfig} from "../slack/routing.ts";
import {parseConfig} from "./config.ts";
test("shared name vectors match protocol.ParseName in both platform configurations",async()=>{
 for(const v of vectors){
  expect(parseBusName(v.input)??null).toBe(v.canonical);
  if(v.canonical!==null){
   expect(parseConfig({route:v.input})).toEqual({aliases:{},route:v.canonical});
   expect((await routingConfig({route:v.input},async s=>s)).route).toBe(v.canonical);
  }
 }
});
