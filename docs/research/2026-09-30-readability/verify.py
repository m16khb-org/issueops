"""One-off reproduction and evidence checks for the ten frozen public bodies."""
import hashlib
import json
import pathlib
import re
import statistics
import sys

ROOT = pathlib.Path(__file__).resolve().parent
REPO = ROOT.parents[2]
FROZEN_SHA = '4d01bf05760d7aec4d5f509cac21af66eb5f494325845dd7c056bf9d4f0e3287'
TERMS = ('execution', 'lease', '봉인', 'generation')
QUESTIONS = ('무엇이 문제이고 무엇이 바뀌는가?', '왜 지금 필요한가?', '끝났는지 어떻게 확인하는가?', '모르는 용어나 두 번 읽은 문장이 있는가?')


def read(name):
    return json.loads((ROOT / name).read_text())


def sha(text):
    return hashlib.sha256(text.encode('utf-8')).hexdigest()


def headings(body):
    # The current Go bodySections contract toggles on either fence prefix.
    fence = False
    titles = []
    for line in body.split('\n'):
        trimmed = line.strip()
        if trimmed.startswith(('```', '~~~')):
            fence = not fence
        if not fence and trimmed.startswith('## '):
            titles.append(trimmed[3:].strip())
    return titles


def metric(sample):
    body = sample['body']
    titles = headings(body)
    # Match artifactreadability.blankCode: triple-backtick blocks, inline code.
    prose = re.sub(r'(?s)```.*?```', '', body)
    prose = re.sub(r'`[^`]*`', '', prose)
    completion = re.search(r'<!-- issueops:completion:start -->(.*?)<!-- issueops:completion:end -->', body, re.S)
    return {'number': sample['number'], 'h2_sections': len(titles), 'unicode_characters': len(body),
            'terms_raw': {term: body.count(term) for term in TERMS},
            'first_h2_summary': bool(titles and titles[0] == '요약'),
            'hex64_outside_code': len(re.findall(r'\b[0-9a-fA-F]{64}\b', prose)),
            'completion_characters': len(completion.group(0)) if completion else 0}


def metrics():
    rows = [metric(s) for s in read('frozen-sample.json')['samples']]
    return {'method': 'reconstructed, not verified equivalent to the 2026-09-23 method', 'denominator': 10,
            'rows': rows, 'totals': {key: sum(r[key] for r in rows) for key in ('h2_sections', 'unicode_characters', 'hex64_outside_code', 'completion_characters')},
            'medians': {key: statistics.median(r[key] for r in rows) for key in ('h2_sections', 'unicode_characters')},
            'terms_totals': {t: sum(r['terms_raw'][t] for r in rows) for t in TERMS}}


def verify_snapshot(bundle):
    samples = bundle['samples']
    assert [s['number'] for s in samples] == list(range(518, 528))
    assert samples == sorted(samples, key=lambda s: (s['created_at'], s['number']))
    for s in samples:
        assert sha(s['body']) == s['body_sha256']
        assert s['created_at'] > bundle['merged_at']
        assert headings(s['body'])[0] == '요약'
        assert '/Users/' not in s['body'] and '/home/' not in s['body']


def check(mode):
    if mode == 'snapshot':
        assert hashlib.sha256((ROOT / 'frozen-sample.json').read_bytes()).hexdigest() == FROZEN_SHA
        verify_snapshot(read('frozen-sample.json'))
        selection = read('selection.json')
        candidates = sorted(selection['candidates'], key=lambda r: (r['created_at'], r['number']))
        eligible = [r['number'] for r in candidates if r['created_at'] > selection['merge_at'] and r['first_h2'] == '요약']
        assert eligible[:10] == list(range(518, 528)) and eligible[10] == 528
        print('snapshot verified: 10')
    elif mode == 'metrics':
        assert metrics() == read('metrics.json')
        print('metrics reproduced: 10')
    elif mode == 'evidence':
        contexts = set()
        rubric_hash = hashlib.sha256((ROOT / 'pre-reader-rubric.json').read_bytes()).hexdigest()
        verdicts = read('reader-comparison.json')['samples']
        assert [v['number'] for v in verdicts] == list(range(518, 528))
        rubric = read('pre-reader-rubric.json')
        for original in rubric['samples']:
            if original['original_intent'] == 'available':
                assert sha(original['intent_text']) == original['intent_sha256']
        for s, v, original in zip(read('frozen-sample.json')['samples'], verdicts, rubric['samples']):
            r = read(f"readers/{s['number']}.json")
            assert r['context_id'] not in contexts
            contexts.add(r['context_id'])
            assert r['body_sha256'] == s['body_sha256'] and sha(r['prompt']) == r['prompt_sha256']
            expected = '제목: '+s['title']+'\n\n본문:\n'+s['body']+'\n\n본문만 읽고 도구나 외부 자료를 쓰지 말 것.\n'+'\n'.join(f'{i}. {q}' for i, q in enumerate(QUESTIONS, 1))
            assert r['prompt'] == expected
            assert r['pre_reader_rubric_sha256'] == rubric_hash
            assert r['started_at'] > rubric['frozen_at']
            assert r['tool_calls'] == 0 and r['exit_code'] == 0 and r['attempt'] == 1
            assert r['answer'].strip() and len(v['answers']) == 4
            assert v['original_intent'] == original['original_intent']
            for answer in v['answers']:
                assert answer['evidence'] and answer['verdict'] in ('match', 'mismatch', 'unanswerable', 'unknown', 'terminology')
                assert answer['reader_quote'] in r['answer']
        print('reader evidence verified: 10')
    elif mode == 'remote':
        rows = read('remote-comparison.json')['samples']
        assert [r['number'] for r in rows] == list(range(518, 528))
        for s, r in zip(read('frozen-sample.json')['samples'], rows):
            assert r['snapshot_sha256'] == s['body_sha256']
            assert sha(r['live_body']) == r['live_sha256']
            assert r['drift'] == (r['snapshot_sha256'] != r['live_sha256'])
        print('remote comparison recorded: 10')
    elif mode == 'decision':
        rel = read('decision.json')['adr']
        adr = REPO / rel
        assert adr.is_file() and '권장 절차로 유지' in adr.read_text()
        assert adr.name in (REPO / '.issueops/ADR.md').read_text()
        assert (ROOT / 'README.md').is_file()
        for doc in (adr, ROOT / 'README.md'):
            for link in re.findall(r'\]\(([^)]+)\)', doc.read_text()):
                if not link.startswith(('https://', 'http://', '#')):
                    assert (doc.parent / link.split('#')[0]).exists(), (doc, link)
        print('decision linked')
    elif mode == 'fixtures':
        bundle = read('frozen-sample.json')
        bundle['samples'][0]['body'] += 'x'
        try:
            verify_snapshot(bundle)
        except AssertionError:
            pass
        else:
            raise AssertionError('tampered snapshot accepted')
        h = 'a' * 64
        body = f'## 요약\n```md\n## fake\n{h}\n```\n`{h}`\n{h}\n'
        row = metric({'number': 0, 'body': body})
        assert row['h2_sections'] == 1 and row['hex64_outside_code'] == 1
        assert metric({'number':0,'body':'x'+h+'x'})['hex64_outside_code'] == 0
        print('negative fixtures verified')
    else:
        raise ValueError(mode)


if __name__ == '__main__':
    check(sys.argv[1])
