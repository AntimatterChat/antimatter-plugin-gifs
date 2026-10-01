// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {Client4} from 'mattermost-redux/client';

import GifPost from './components/gif_post';
import PickerRoot from './components/picker_root';
import manifest from './manifest';
import {closePicker, loadConfig, openPicker, resetFavorites} from './state';
import type {GifsAPI, PluginClass, PluginRegistry} from './types/host';

import './styles.css';

// The type of the posts of GIFs and stickers (see server/post.go).
export const POST_TYPE = 'custom_antimatter_gif';

export default class Plugin implements PluginClass {
    public initialize(registry: PluginRegistry) {
        if (window.basename) {
            Client4.setUrl(window.basename);
        }
        loadConfig();

        registry.registerPostTypeComponent(POST_TYPE, GifPost);
        registry.registerRootComponent(PickerRoot);

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
