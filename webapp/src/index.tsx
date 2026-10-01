// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';

import {Client4} from 'mattermost-redux/client';

import GifPost from './components/gif_post';
import {StickerIcon} from './components/icons';
import PickerRoot from './components/picker_root';
import manifest from './manifest';
import {messages} from './messages';
import {closePicker, getClientConfig, loadConfig, openPicker, resetFavorites} from './state';
import type {GifsAPI, PluginClass, PluginRegistry, SlashCommandArgs, SlashCommandResult} from './types/host';
import type {Kind} from './types/gifs';

import './styles.css';

// The type of the posts of GIFs and stickers (see server/post.go).
export const POST_TYPE = 'custom_antimatter_gif';

const COMMAND = /^\/(gif|sticker)(?:\s+([\s\S]*))?$/;

// openFromCommand opens the picker for "/gif <search>" and "/sticker <search>", instead of the
// server's preview. The command goes to the server when GIFs aren't available here.
export function openFromCommand(message: string, args: SlashCommandArgs): SlashCommandResult {
    const match = COMMAND.exec(message.trim());
    const config = getClientConfig();
    if (!match || !config?.configured || (match[1] === 'sticker' && !config.stickers)) {
        return {message, args};
    }
    openPicker({channelId: args.channel_id, rootId: args.root_id, tab: match[1] as Kind, query: (match[2] || '').trim()});
    return {};
}

export default class Plugin implements PluginClass {
    public initialize(registry: PluginRegistry) {
        if (window.basename) {
            Client4.setUrl(window.basename);
        }
        loadConfig();

        registry.registerPostTypeComponent(POST_TYPE, GifPost);
        registry.registerRootComponent(PickerRoot);
        registry.registerSlashCommandWillBePostedHook(openFromCommand);

        if (typeof registry.registerComposerMenuItem === 'function') {
            // An item of the message box's "+" menu, under Fusion with its sticker icon.
            registry.registerComposerMenuItem({
                text: <FormattedMessage {...messages.menuItem}/>,
                icon: <StickerIcon className='amgifs-menu-icon'/>,
                fusionIcon: 'sticker',
                action: ({channelId, rootId}) => openPicker({channelId, rootId}),
                shouldRender: () => Boolean(getClientConfig()?.configured),
            });
        } else {
            // Hosts without the "+" menu hook get a channel header button (and the commands).
            registry.registerChannelHeaderButtonAction(
                <StickerIcon className='amgifs-header-icon'/>,
                (channel) => openPicker({channelId: channel.id}),
                <FormattedMessage {...messages.headerButton}/>,
                <FormattedMessage {...messages.headerButton}/>,
            );
        }

        // Let the host open the picker, e.g. from a tab of its emoji picker.
        const api: GifsAPI = {openPicker};
        window.antimatterGifs = api;
    }

    public uninitialize() {
        closePicker();
        resetFavorites();
        delete window.antimatterGifs;
    }
}

window.registerPlugin(manifest.id, new Plugin());
