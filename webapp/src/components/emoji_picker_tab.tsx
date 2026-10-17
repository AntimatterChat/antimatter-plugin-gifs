// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';

import {loadConfig} from '../state';
import type {Kind} from '../types/gifs';
import type {EmojiPickerTabProps} from '../types/host';

import {PanelWithViews} from './manage_views';
import {PickerPanel} from './picker';

// EmojiPickerTab is the GIFs or Stickers tab of the host's emoji picker (registerEmojiPickerTab), as
// in the Fusion UI mockup's picker: the picker's panel, posting to the message box's channel or
// thread and closing the host's picker once posted. Under Fusion, it renders the Fusion markup in
// the picker's body; in the classic emoji picker, the plugin's own markup in a tab of fixed height.
// A Fusion host that hands its picker kit gets the add and manage views inside the picker; the
// others keep the button opening the upload dialog.
function EmojiPickerTab({kind, channelId, rootId, filter, onFilterChange, onSelectDone, isFusion, pickerKit}: EmojiPickerTabProps & {kind: Kind}) {
    useEffect(() => {
        loadConfig();
    }, []);

    if (!channelId) {
        return null;
    }

    if (isFusion && pickerKit) {
        return (
            <PanelWithViews
                kit={pickerKit}
                kind={kind}
                channelId={channelId}
                rootId={rootId}
                query={filter}
                onQuery={onFilterChange}
                onPosted={onSelectDone}
                onClose={onSelectDone}
            />
        );
    }

    const panel = (
        <PickerPanel
            kind={kind}
            fusion={isFusion}
            channelId={channelId}
            rootId={rootId}
            query={filter}
            onQuery={onFilterChange}
            onPosted={onSelectDone}
            onClose={onSelectDone}
        />
    );
    return isFusion ? panel : <div className='amgifs-tab'>{panel}</div>;
}

export function GifsTab(props: EmojiPickerTabProps) {
    return (
        <EmojiPickerTab
            {...props}
            kind='gif'
        />
    );
}

export function StickersTab(props: EmojiPickerTabProps) {
    return (
        <EmojiPickerTab
            {...props}
            kind='sticker'
        />
    );
}
