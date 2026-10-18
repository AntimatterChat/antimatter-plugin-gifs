// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import fs from 'fs';
import path from 'path';

const css = fs.readFileSync(path.join(__dirname, 'styles.css'), 'utf8');

// The rules that size the previews in the drop zone of the Fusion picker's add view.
const inDropZone = css.split('\n').filter((l) => l.startsWith(':is(.am-app, .am-layer) .am-pv-drop .amgifs-add-preview'));
const pixels = (line: string, prop: string) => Number(new RegExp(`(?:\\{|;)\\s*${prop}:\\s*(\\d+(?:\\.\\d+)?)px`).exec(line)?.[1] ?? NaN);

describe('the add view fits the Fusion picker without scrolling', () => {
    // The host's drop zone gives a preview 72px: the media and their captions stay under it.
    test.each([
        ['gif', 'picker'],
        ['gif', 'message'],
        ['sticker', 'picker'],
        ['sticker', 'message'],
    ])('the %s preview at the size of a %s stays as low as the drop zone', (kind, where) => {
        const line = inDropZone.find((l) => l.includes(`.amgifs-add-preview-${kind} .amgifs-add-preview-${where} {`));
        expect(line).toBeDefined();
        const height = pixels(line!, 'max-height') || pixels(line!, 'height');
        expect(height).toBeLessThanOrEqual(56);
    });

    test('the captions are small, close to the media', () => {
        const captions = inDropZone.find((l) => l.includes('figcaption {'));
        expect(pixels(captions!, 'line-height')).toBeLessThanOrEqual(14);
        const figure = inDropZone.find((l) => l.includes('figure {'));
        expect(pixels(figure!, 'gap')).toBeLessThanOrEqual(4);
    });
});
