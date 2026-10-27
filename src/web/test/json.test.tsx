import { expect, test } from "bun:test";
import { h } from "../jsx.ts";
import { Json } from "../ui/json.tsx";

const decode = (s: string) => s.replace(/<[^>]*>/g, "").replace(/&quot;/g,'"').replace(/&#39;/g,"'").replace(/&lt;/g,"<").replace(/&gt;/g,">").replace(/&amp;/g,"&");
test("JSON colors every token kind while copying exact escaped text", () => {
 const value = '{\n  "key": "</span><script> & \\\"quoted\\\"", "n": -1.2e+3, "b": true, "nil": null\n}';
 const html = String(<Json value={value} />);
 for(const kind of ["key","string","number","literal","punctuation"]) expect(html).toContain(`<span class="json-${kind}">`);
 expect(html).not.toContain('<script>');expect(html).toContain('&lt;/span&gt;&lt;script&gt;');
 expect(decode(html)).toBe(value);
 expect(String(<Json value={'not JSON <script>'} />)).toBe('not JSON &lt;script&gt;');
});
