import {expect, test} from 'bun:test';
import {normalizeTags} from './tags.ts';

test('record labels reject the no-tags listing sentinel', () => {
  for (const labels of [['no-tags'], ['review,no-tags'], ['NO-TAGS']])
    expect(() => normalizeTags(labels)).toThrow('listing filter');
  expect(normalizeTags(['Review,PERSONAL', 'review'])).toEqual(['personal', 'review']);
  expect(normalizeTags([])).toEqual([]);
  expect(normalizeTags([''])).toEqual([]);
});
