"""Readable output and injection-resistant Markdown without remote resources."""
from pathlib import Path
import sys
import unittest
HERE=Path(__file__).resolve().parent;sys.path.insert(0,str(HERE))
from extract import extract,WARNING,OUTPUT_MAX
from transport import ContentError


class Extraction(unittest.TestCase):
    def render(self,source,format='text',mime='text/html; charset=utf-8'):
        return extract(source.encode(),mime,format,'https://public.example/page')
    def test_readable_text_and_warning_without_title_or_hidden_content(self):
        page='<head><title>Hidden title</title><script>headSecret</script></head><h1>A heading</h1><p>Hello <b>world</b> &amp; friends.</p><ul><li>One<li>Two</ul><div hidden>hiddenSecret</div><p aria-hidden="true">ariaSecret</p><div style="display: none !important">styleSecret</div><noscript>noSecret</noscript><template><p>templateSecret</p></template>'
        result=self.render(page)
        self.assertEqual(result,WARNING+'A heading\n\nHello world & friends.\n\nOne\n\nTwo')
        self.assertNotIn('Secret',result);self.assertNotIn('title',result)
    def test_markdown_has_safe_links_and_escapes_untrusted_syntax(self):
        page='<base href="http://127.0.0.1/"><h2>Title</h2><p>[x](javascript:alert(1)) <a href="/ok?q=(a)">Good</a> <a href="  JaVaScRiPt:alert(1)">Bad</a> <a href="data:text/html,evil">Data</a> <a href="vbscript:evil">VB</a> &lt;script&gt;</p>'
        result=self.render(page,'markdown')
        self.assertTrue(result.startswith(WARNING+'## Title'))
        self.assertIn('[Good](https://public.example/ok?q=%28a%29)',result)
        self.assertNotIn('](javascript:',result);self.assertNotIn('](data:',result);self.assertNotIn('](vbscript:',result)
        self.assertIn('\\[x\\]\\(javascript:',result);self.assertIn('\\<script\\>',result);self.assertNotIn('127.0.0.1',result)
    def test_entities_are_decoded_once_and_blocks_do_not_join_words(self):
        self.assertEqual(self.render('<p>one</p><p>two &amp;lt;b&amp;gt;</p>'),WARNING+'one\n\ntwo &lt;b&gt;')
        self.assertEqual(self.render('<p>hel<b>lo</b> world</p>'),WARNING+'hello world')
    def test_hidden_ancestor_cannot_be_popped_by_malformed_optional_tags(self):
        result=self.render('<p>visible</p><template><li>secret<li>moreSecret</template><p>after</p>')
        self.assertEqual(result,WARNING+'visible\n\nafter')
    def test_plaintext_charset_and_bom_are_explicit(self):
        self.assertEqual(extract(b'\xef\xbb\xbfhello','text/plain','text','https://public.example/'),WARNING+'hello')
        self.assertEqual(extract('é'.encode(),'text/plain; charset=iso-8859-1','text','https://public.example/'),WARNING+'Ã©')
        self.assertEqual(extract(b'\xe9','text/plain; charset=windows-1252','text','https://public.example/'),WARNING+'é')
        for body,mime in [(b'\xef\xbb\xbfhello','text/plain; charset=latin1'),(b'\xff','text/plain'),(b'hello\x00','text/plain'),(b'x','application/octet-stream'),(b'x','text/plain; charset=utf-16')]:
            with self.subTest(mime=mime),self.assertRaises(ContentError):extract(body,mime,'text','https://public.example/')
    def test_text_markdown_is_escaped_and_never_becomes_raw_html(self):
        self.assertEqual(self.render('[x](javascript:a) <b>x</b>','markdown','text/markdown'),WARNING+'\\[x\\]\\(javascript:a\\) \\<b\\>x\\</b\\>')
    def test_nesting_nodes_attributes_text_and_output_have_finite_caps(self):
        for source in ('<table>'*65,'<br>'*50001,'<a x="'+'a'*8193+'">x</a>','<p>'+'x'*65537+'</p>'):
            with self.subTest(size=len(source)),self.assertRaises(ContentError):self.render(source)
        with self.assertRaises(ContentError):self.render('x'*(OUTPUT_MAX-len(WARNING)+1),'text','text/plain')
        result=self.render('x'*(OUTPUT_MAX-len(WARNING)),'text','text/plain');self.assertEqual(len(result.encode()),OUTPUT_MAX)
    def test_whitespace_work_and_unclosed_tags_are_bounded(self):
        self.assertEqual(self.render('<p>'+' '*65000+'ok</p>'),WARNING+'ok')
        with self.assertRaises(ContentError):self.render('<span>'*10000)


if __name__=='__main__':unittest.main()
