// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import * as client from './client';
import {getPicker, loadConfig} from './state';
import type {PluginClass, PluginRegistry} from './types/host';

jest.mock('./client', () => ({
    getConfig: jest.fn(() => Promise.resolve({configured: true, stickers: true})),
}));

let Plugin: new () => PluginClass;
let openFromCommand: typeof import('./index').openFromCommand;
beforeAll(() => {
    window.registerPlugin = jest.fn();
    // eslint-disable-next-line global-require
    const index = require('./index');
    Plugin = index.default;
    openFromCommand = index.openFromCommand;
});

function makeRegistry(withComposerMenu: boolean) {
    const registry = {
        registerRootComponent: jest.fn(),
        registerPostTypeComponent: jest.fn(),
        registerChannelHeaderButtonAction: jest.fn(),
        registerSlashCommandWillBePostedHook: jest.fn(),
    } as Record<string, jest.Mock>;
    if (withComposerMenu) {
        registry.registerComposerMenuItem = jest.fn();
    }
    return registry;
}

test('registers the post type, the picker and the composer menu item', async () => {
    const registry = makeRegistry(true);
    new Plugin().initialize(registry as unknown as PluginRegistry, {} as never);
    expect(window.registerPlugin).toHaveBeenCalledWith('com.antimatterchat.gifs', expect.anything());
    expect(registry.registerPostTypeComponent).toHaveBeenCalledWith('custom_antimatter_gif', expect.any(Function));
    expect(registry.registerRootComponent).toHaveBeenCalled();
    expect(registry.registerChannelHeaderButtonAction).not.toHaveBeenCalled();

    const item = registry.registerComposerMenuItem.mock.calls[0][0];
    expect(item.fusionIcon).toBe('sticker');
    await loadConfig();
    expect(item.shouldRender({}, {channelId: 'c'})).toBe(true);
    item.action({channelId: 'c', rootId: 'r'});
    expect(getPicker()).toEqual({channelId: 'c', rootId: 'r', tab: 'gif', query: ''});
    expect(typeof window.antimatterGifs?.openPicker).toBe('function');
});

test('falls back to a channel header button', () => {
    const registry = makeRegistry(false);
    new Plugin().initialize(registry as unknown as PluginRegistry, {} as never);
    const [, action] = registry.registerChannelHeaderButtonAction.mock.calls[0];
    action({id: 'channel2'});
    expect(getPicker()).toMatchObject({channelId: 'channel2', tab: 'gif'});
});

test('/gif and /sticker open the picker', async () => {
    await loadConfig();
    const args = {channel_id: 'c1', root_id: 'r1'};
    expect(openFromCommand('/gif happy cat ', args)).toEqual({});
    expect(getPicker()).toEqual({channelId: 'c1', rootId: 'r1', tab: 'gif', query: 'happy cat'});
    expect(openFromCommand('/sticker', args)).toEqual({});
    expect(getPicker()).toMatchObject({tab: 'sticker', query: ''});
    expect(openFromCommand('/gifs x', args)).toEqual({message: '/gifs x', args});
    expect(openFromCommand('/away', args)).toEqual({message: '/away', args});

    // Without stickers, or GIFs, the commands go to the server.
    (client.getConfig as jest.Mock).mockResolvedValue({configured: true, stickers: false});
    await loadConfig();
    expect(openFromCommand('/sticker x', args)).toEqual({message: '/sticker x', args});
});
