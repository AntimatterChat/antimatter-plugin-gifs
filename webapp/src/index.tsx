// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';

import {Client4} from 'mattermost-redux/client';

import {GifsTab, StickersTab} from './components/emoji_picker_tab';
import GifPost from './components/gif_post';
import {GifIcon, StickerIcon} from './components/icons';
import PickerRoot from './components/picker_root';
import UploadDialogRoot from './components/upload_dialog';
import manifest from './manifest';
import {messages} from './messages';
import {closePicker, closeUpload, getClientConfig, loadConfig, openPicker, openUpload, resetFavorites} from './state';
import type {GifsAPI, PluginClass, PluginRegistry, SlashCommandArgs, SlashCommandResult} from './types/host';
import type {Kind} from './types/gifs';
import {registerTranslations} from './translations';

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

// registerIntegrationsOptions adds "Custom GIFs" and "Custom stickers" to the host's integrations
// pages, next to custom emoji, for the users who can add some: they open the upload dialog. The
// configuration is loaded by then, so the entries show as soon as they're registered.
export function registerIntegrationsOptions(registry: PluginRegistry) {
    const config = getClientConfig();
    if (typeof registry.registerIntegrationsOption !== 'function' || !config?.configured || !config.can_upload) {
        return;
    }
    registry.registerIntegrationsOption({
        title: <FormattedMessage {...messages.customGifs}/>,
        description: <FormattedMessage {...messages.customGifsDescription}/>,
        fusionIcon: 'image',
        action: () => openUpload({kind: 'gif'}),
    });
    if (config.stickers) {
        registry.registerIntegrationsOption({
            title: <FormattedMessage {...messages.customStickers}/>,
            description: <FormattedMessage {...messages.customStickersDescription}/>,
            fusionIcon: 'sticker',
            action: () => openUpload({kind: 'sticker'}),
        });
    }
}

export default class Plugin implements PluginClass {
    public initialize(registry: PluginRegistry) {
        registerTranslations(registry);

        if (window.basename) {
            Client4.setUrl(window.basename);
        }
        loadConfig().then(() => registerIntegrationsOptions(registry));

        registry.registerPostTypeComponent(POST_TYPE, GifPost);
        registry.registerRootComponent(PickerRoot);
        registry.registerRootComponent(UploadDialogRoot);
        registry.registerSlashCommandWillBePostedHook(openFromCommand);

        if (typeof registry.registerEmojiPickerTab === 'function') {
            // GIFs and Stickers tabs of the message box's emoji picker, as in the Fusion UI mockup's
            // picker (GIFs, Stickers, Emoji). The GIFs tab replaces the host's Giphy picker.
            registry.registerEmojiPickerTab({
                id: `${manifest.id}.gifs`,
                label: <FormattedMessage {...messages.tabGifs}/>,
                icon: <GifIcon className='amgifs-tab-icon'/>,
                order: 0,
                component: GifsTab,
                shouldRender: () => Boolean(getClientConfig()?.configured),
                replacesGifPicker: true,
            });
            registry.registerEmojiPickerTab({
                id: `${manifest.id}.stickers`,
                label: <FormattedMessage {...messages.tabStickers}/>,
                icon: <StickerIcon className='amgifs-tab-icon'/>,
                order: 1,
                component: StickersTab,
                shouldRender: () => Boolean(getClientConfig()?.configured && getClientConfig()?.stickers),
            });
        } else if (typeof registry.registerComposerMenuItem === 'function') {
            // Hosts without emoji picker tabs: an item of the message box's "+" menu, under Fusion
            // with its sticker icon.
            registry.registerComposerMenuItem({
                text: <FormattedMessage {...messages.menuItem}/>,
                icon: <StickerIcon className='amgifs-menu-icon'/>,
                fusionIcon: 'sticker',
                action: ({channelId, rootId}) => openPicker({channelId, rootId}),
                shouldRender: () => Boolean(getClientConfig()?.configured),
            });
        } else {
            // Hosts without either hook get a channel header button (and the commands).
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
        closeUpload();
        resetFavorites();
        delete window.antimatterGifs;
    }
}

window.registerPlugin(manifest.id, new Plugin());
