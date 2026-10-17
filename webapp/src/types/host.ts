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

    // Antimatter Fusion: the components of the picker's add and manage views, so that the tab's
    // views look like the emoji tab's and stay inside the picker. Older hosts and the classic UI
    // leave it out: the tab keeps its upload dialog then.
    pickerKit?: PickerKit;
};

// A row of the kit's manage list.
export type PickerListItem = {
    id: string;
    name: string;
    image: React.ReactNode;

    // Who added it and when (ms since the epoch).
    by?: string;
    at?: number;
    canDelete: boolean;
};

// The host's picker kit (Fusion's fusion/popovers/picker_views.tsx, pickerKit), described here by
// its shape since the plugin can't import the host's code.
export type PickerKit = {
    version: number;

    // Which view shows ('grid' or one of the tab's), opening one, and going back to the grid
    // (back(false) leaves the focus where the tab puts it).
    usePickerView: <V extends string>() => [V | 'grid', (view: V) => void, (restoreFocus?: unknown) => void];
    useOnEscape: (active: boolean, onEscape: () => void) => void;
    PickerViewButtons: React.ComponentType<{onAdd?: () => void; onManage?: () => void; addLabel: string; manageLabel: string}>;
    PickerViewFrame: React.ComponentType<{
        title: string;
        onBack: () => void;
        tools?: React.ReactNode;
        actions?: React.ReactNode;
        variant?: 'picker' | 'page';
        noBack?: boolean;
        children: React.ReactNode;
        onSubmit?: () => void;
    }>;
    PickerNotice: React.ComponentType<{kind?: 'error' | 'ok' | 'info'; children: React.ReactNode}>;
    PickerField: React.ComponentType<{
        label: string;
        hint?: React.ReactNode;
        error?: React.ReactNode;
        ok?: React.ReactNode;
        aside?: React.ReactNode;
        children: (ids: {id: string; describedBy: string; invalid: boolean}) => React.ReactNode;
    }>;
    PickerSearch: React.ComponentType<{value: string; onChange: (value: string) => void; placeholder: string; autoFocus?: boolean}>;
    PickerDropZone: React.ComponentType<{
        accept: string;
        file: File | null;
        onFile: (file: File) => void;
        label: string;
        hint: React.ReactNode;
        error?: React.ReactNode;
        preview?: React.ReactNode;
        changeLabel: string;
    }>;
    PickerManageList: React.ComponentType<{
        items: PickerListItem[];
        onDelete: (item: PickerListItem) => Promise<string | void>;
        confirmText: (item: PickerListItem) => string;
        deleteLabel: (item: PickerListItem) => string;
        deletedText: (item: PickerListItem) => string;
        empty: string;
        loading?: boolean;
        hasMore?: boolean;
        onLoadMore?: () => void;
        onOpen?: (item: PickerListItem) => void;
    }>;
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

    // Antimatter: an entry of the integrations pages, next to custom emoji.
    registerIntegrationsOption?(option: {
        title: React.ReactNode;
        description: React.ReactNode;
        fusionIcon?: string;
        action: () => void;
        shouldRender?: (state: GlobalState) => boolean;
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
