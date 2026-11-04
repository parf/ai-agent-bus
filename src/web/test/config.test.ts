import {expect, test} from "bun:test";
import {defaultSocket,load} from "../config.ts";

test("web configuration has one installed socket and one TLS directory form",()=>{
  expect(defaultSocket({})).toBe("/run/agent-bus/bus.sock");
  expect(defaultSocket({XDG_RUNTIME_DIR:"/run/user/123"})).toBe("/run/user/123/agent-bus/bus.sock");
  expect(load({AGENT_BUS_ADDR:"/private/bus.sock"}).daemon).toBe("/private/bus.sock");
  for(const name of ["AGENT_BUS_WEB_CERT","AGENT_BUS_WEB_KEY","AGENT_BUS_WEB_RUNTIME"]){
    for(const value of ["","/private/file"]){
      expect(()=>load({[name]:value})).toThrow(`${name} is not supported`);
      expect(()=>load({[name]:value,AGENT_BUS_WEB_TLS_DIR:"/missing"})).toThrow(`${name} is not supported`);
    }
  }
});
