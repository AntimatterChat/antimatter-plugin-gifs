// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import type React from 'react';
import type {Store} from 'redux';

import type {Channel} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';
import type {GlobalState} from '@mattermost/types/store';

import type {WebUI} from '../web_ui';

export type ComposerMenuItemContext = {
    channelId: string;
    rootId?: string;
};

// Antimatter: the props of a tab of the host's emoji picker.
export type EmojiPickerTabProps = {
    channelId?: string;
    rootId?: string;

    // The text of the picker's search box, which the tab renders.
    filter: string;
    onFilterChange: (filter: string) => void;

    // Closes the picker.
    onSelectDone: () => void;

    // Whether the Fusion UI renders the tab.
    isFusion: boolean;
    insertText?: (text: string) => void;
};

export type SlashCommandArgs = {
    channel_id: string;
    team_id?: string;
    root_id?: string;
};

export type SlashCommandResult = {message?: string; args?: SlashCommandArgs; error?: {message: string}};

// The subset of the host's plugin registry this plugin uses. The hooks that older or upstream
// hosts may lack are optional: the plugin checks for them and degrades gracefully.
export interface PluginRegistry {
    registerRootComponent(component: React.ComponentType): string;
    registerPostTypeComponent(type: string, component: React.ComponentType<{post: Post}>): string;
    registerChannelHeaderButtonAction(icon: React.ReactNode, action: (channel: Channel) => void, dropdownText: React.ReactNode, tooltipText: React.ReactNode): string;
    registerSlashCommandWillBePostedHook(hook: (message: string, args: SlashCommandArgs) => SlashCommandResult | Promise<SlashCommandResult>): string;

    // Antimatter: an item of the message box's "+" menu.
    registerComposerMenuItem?(item: {
        text: React.ReactNode;
        icon: React.ReactNode;
        fusionIcon?: string;
        action: (ctx: ComposerMenuItemContext) => void;
        shouldRender?: (state: GlobalState, ctx: ComposerMenuItemContext) => boolean;
    }): string;

    // Antimatter: a tab of the message box's emoji picker.
    registerEmojiPickerTab?(tab: {
        id?: string;
        label: React.ReactNode;
        icon?: React.ReactNode;
        fusionIcon?: string;
        order?: number;
        component: React.ComponentType<EmojiPickerTabProps>;
        shouldRender?: (state: GlobalState, ctx: ComposerMenuItemContext) => boolean;
        replacesGifPicker?: boolean;
    }): string;
}

export type PluginStore = Store<GlobalState>;

export interface PluginClass {
    initialize(registry: PluginRegistry, store: PluginStore): void | Promise<void>;
    uninitialize?(): void;
}

// The API the plugin exposes on window.antimatterGifs, e.g. for an emoji picker tab of the host.
export type GifsAPI = {
    openPicker(options: {channelId: string; rootId?: string; tab?: 'gif' | 'sticker'; query?: string}): void;
};

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: PluginClass): void;
        basename?: string;

        // Set by the Antimatter web UIs before plugins load
        antimatterWebUI?: WebUI;

        // Set by this plugin
        antimatterGifs?: GifsAPI;
    }
}
