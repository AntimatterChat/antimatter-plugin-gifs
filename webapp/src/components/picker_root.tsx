// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useLayoutEffect, useRef, useState} from 'react';
import ReactDOM from 'react-dom';

import {closePicker, loadConfig, usePicker, type PickerRequest} from '../state';
import {isFusionUI} from '../web_ui';

import Picker from './picker';

const MARGIN = 8;

// anchorRect is the message box the picker posts to: the thread's box for replies, else the
// channel's, else the focused text box.
function anchorRect(rootId?: string): DOMRect | null {
    const ids = rootId ? ['reply_textbox', 'post_textbox'] : ['post_textbox'];
    for (const id of ids) {
        const el = document.getElementById(id);
        if (el && el.getClientRects().length > 0) {
            return el.getBoundingClientRect();
        }
    }
    const active = document.activeElement;
    if (active instanceof HTMLTextAreaElement) {
        return active.getBoundingClientRect();
    }
    return null;
}

// place puts the picker above the message box, right-aligned with it, like the mockup's picker
// above the composer; centered without one.
export function place(pop: {offsetWidth: number; offsetHeight: number}, rect: DOMRect | null, vw: number, vh: number) {
    const w = pop.offsetWidth;
    const h = pop.offsetHeight;
    let left = (vw - w) / 2;
    let top = (vh - h) / 2;
    if (rect) {
        left = rect.right - w;
        top = rect.top - h - MARGIN;
        if (top < MARGIN) {
            top = rect.bottom + MARGIN;
        }
    }
    const clamp = (v: number, size: number, max: number) => Math.max(MARGIN, Math.min(v, max - size - MARGIN));
    return {left: clamp(left, w, vw), top: clamp(top, h, vh)};
}

// layerContainer is where the picker and dialogs render: Fusion's layer (its popovers' container,
// whose am-layer class scopes the Fusion styles), else the body.
export function layerContainer(fusion: boolean): HTMLElement {
    if (!fusion) {
        return document.body;
    }
    const layer = document.getElementById('am-layer');
    if (layer) {
        return layer;
    }
    let own = document.getElementById('amgifs-layer');
    if (!own) {
        own = document.createElement('div');
        own.id = 'amgifs-layer';
        own.className = 'am-layer';
        document.body.appendChild(own);
    }
    return own;
}

function PickerPopover({request}: {request: PickerRequest}) {
    const fusion = isFusionUI();
    const ref = useRef<HTMLDivElement>(null);
    const [position, setPosition] = useState<{left: number; top: number} | null>(null);
    const onClose = useCallback(() => closePicker(), []);

    useEffect(() => {
        loadConfig();
    }, []);

    useLayoutEffect(() => {
        const update = () => {
            if (ref.current) {
                setPosition(place(ref.current, anchorRect(request.rootId), window.innerWidth, window.innerHeight));
            }
        };
        update();
        window.addEventListener('resize', update);
        return () => window.removeEventListener('resize', update);
    }, [request.rootId]);

    useEffect(() => {
        const onPointerDown = (e: PointerEvent) => {
            if (!ref.current?.contains(e.target as Node)) {
                closePicker();
            }
        };
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.stopPropagation();
                closePicker();
            }
        };
        document.addEventListener('pointerdown', onPointerDown, true);
        document.addEventListener('keydown', onKeyDown, true);
        return () => {
            document.removeEventListener('pointerdown', onPointerDown, true);
            document.removeEventListener('keydown', onKeyDown, true);
        };
    }, []);

    const picker = (
        <Picker
            request={request}
            fusion={fusion}
            onClose={onClose}
            popRef={ref}
            style={position || {left: -9999, top: -9999}}
        />
    );
    return ReactDOM.createPortal(fusion ? picker : <div className='amgifs-layer'>{picker}</div>, layerContainer(fusion));
}

// PickerRoot shows the picker while it's open (see openPicker); registered as a root component.
export default function PickerRoot() {
    const request = usePicker();
    if (!request) {
        return null;
    }

    // A new request (another channel, the /gif command) starts a new picker.
    return (
        <PickerPopover
            key={`${request.channelId}:${request.rootId}:${request.tab}:${request.query}`}
            request={request}
        />
    );
}
