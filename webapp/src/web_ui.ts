// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

// Antimatter ships two web UIs: the classic one and Fusion. Under Fusion, the picker and the posts
// use the markup and classes of the Fusion UI (its mockup's GIF and sticker picker). Both UIs say
// which one is running before plugins load.

export type WebUI = 'classic' | 'fusion';

// isFusionUI returns whether the Fusion web UI is running.
export function isFusionUI(): boolean {
    return window.antimatterWebUI === 'fusion' || document.documentElement.dataset.amWebUi === 'fusion';
}

// classNames returns a function prefixing class names with am- under Fusion (its own classes) and
// amgifs- under the classic UI (this plugin's styles.css).
export function classNames(fusion: boolean) {
    const prefix = fusion ? 'am-' : 'amgifs-';
    return (...names: Array<string | false | null | undefined>) => names.filter(Boolean).map((n) => prefix + n).join(' ');
}
