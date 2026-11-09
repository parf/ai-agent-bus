"""R.7 rerank mutants: each must compile and fail test_retrieval on an assertion; the file is restored after each."""
import subprocess, sys, os, pathlib
RAG = pathlib.Path('/usr/local/src/ai-agent-bus/tmp/ab-w1a-R7/src/agents/rag')
M = [
 ('skip the reorder', 'core.py', "        candidates[:]=[candidates[i] for i in order]\n", "        pass\n"),
 ('accept an out-of-range index', 'core.py', "if type(index) is not int or not 0<=index<len(candidates) or index in scores:", "if type(index) is not int or index in scores:"),
 ('call rerank when off', 'core.py', "        if not v.get('rerank'):return {'mode':mode,'records':ranked[:v['limit']]}\n", "        if not v.get('rerank'):\n            self._rerank(v['query'],ranked[:want]);return {'mode':mode,'records':ranked[:v['limit']]}\n"),
 ('no candidate cap', 'core.py', "        candidates=ranked[:want]\n", "        candidates=ranked\n"),
 ('provider text as the reason', 'core.py', "        except RagError:return 'provider_unavailable'\n", "        except RagError as e:return 'provider_unavailable: '+str(e)\n"),
 ('scores written before every row is checked', 'core.py', "                scores[index]=score\n", "                scores[index]=score;candidates[index]['scores']['rerank']=scores[index]\n"),
 ('huge int converted unguarded', 'core.py', "        try:value=float(value)\n        except OverflowError:return None\n", "        value=float(value)\n"),
 ('huge int checked with isfinite (the original)', 'core.py', "                score=finite(score)\n", "                score=score if type(score) in (int,float) and math.isfinite(score) else None\n"),
 ('bool accepted as a score', 'core.py', "    return value if type(value) is float and math.isfinite(value) else None\n", "    if type(value) is bool:value=float(value)\n    return value if type(value) is float and math.isfinite(value) else None\n"),
 ('model mismatch ignored', 'core.py', "            if 'model' in answer and answer['model']!=self.cfg['rerank_model']:return 'model_mismatch'\n", ""),
 ('no model check before paying', 'core.py', "            if not self.cfg['rerank_model']:raise NamedRefusal('rerank_unavailable','rerank_model is not configured')\n", ""),
]
env = {k: v for k, v in os.environ.items() if k not in ('AGENT_BUS_SESSION_FILE', 'AGENT_BUS_TOKEN')}
env['PYTHONDONTWRITEBYTECODE'] = '1'
bad = 0
for name, f, old, new in M:
    p = RAG / f; src = p.read_text()
    assert src.count(old) == 1, name
    p.write_text(src.replace(old, new))
    try:
        c = subprocess.run([sys.executable, '-m', 'py_compile', str(p)], capture_output=True)
        assert c.returncode == 0, name + ' does not compile'
        r = subprocess.run([sys.executable, '-m', 'unittest', 'test_retrieval'], cwd=RAG, env=env, capture_output=True, text=True, timeout=120)
        out = r.stdout + r.stderr
        failed = [l for l in out.splitlines() if l.startswith(('FAIL:', 'ERROR:'))]
        killed = r.returncode != 0 and 'AssertionError' in out
        print(('KILLED ' if killed else 'SURVIVED ') + name + ' :: ' + '; '.join(failed[:3]))
        bad += not killed
    finally:
        p.write_text(src)
sys.exit(1 if bad else 0)
