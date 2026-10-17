// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import fs from 'fs';
import path from 'path';

// The app's icon in the System Console and the app lists (plugin.json's icon_path).
const ICON = path.resolve(__dirname, '..', '..', 'assets', 'icon.svg');

test('the app icon is drawn in Antimatter\'s colours, not Mattermost\'s blue', () => {
    const svg = fs.readFileSync(ICON, 'utf8');
    expect(svg).toMatch(/^<svg /);
    expect(svg).toContain('stroke="#22D3EE"');
    expect(svg.toLowerCase()).not.toMatch(/#(1c58d9|166de0|1e325c|145dbf|2389d7)/);
});
