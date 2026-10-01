// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {classNames, isFusionUI} from './web_ui';

describe('isFusionUI', () => {
    afterEach(() => {
        delete window.antimatterWebUI;
        delete document.documentElement.dataset.amWebUi;
    });

    test('false by default and under the classic UI', () => {
        expect(isFusionUI()).toBe(false);
        window.antimatterWebUI = 'classic';
        expect(isFusionUI()).toBe(false);
    });

    test('true under Fusion', () => {
        window.antimatterWebUI = 'fusion';
        expect(isFusionUI()).toBe(true);
        delete window.antimatterWebUI;
        document.documentElement.dataset.amWebUi = 'fusion';
        expect(isFusionUI()).toBe(true);
    });
});

test('classNames prefixes the classes of each UI', () => {
    expect(classNames(true)('pop', 'picker')).toBe('am-pop am-picker');
    expect(classNames(false)('gif-star', false, 'on')).toBe('amgifs-gif-star amgifs-on');
    expect(classNames(false)(null)).toBe('');
});
