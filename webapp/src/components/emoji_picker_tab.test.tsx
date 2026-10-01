// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {fireEvent, screen, waitFor} from '@testing-library/react';
import React from 'react';

import {makeGif, makeSticker, renderWithProviders} from '../../tests/utils';
import * as client from '../client';
import {loadConfig, resetFavorites} from '../state';

import {GifsTab, StickersTab} from './emoji_picker_tab';

jest.mock('../client', () => ({
    getConfig: jest.fn(() => Promise.resolve({configured: true, stickers: true})),
    featured: jest.fn(),
    search: jest.fn(),
    categories: jest.fn(() => Promise.resolve([])),
    autocomplete: jest.fn(() => Promise.resolve([])),
    getFavorites: jest.fn(() => Promise.resolve([])),
    addFavorite: jest.fn(),
    removeFavorite: jest.fn(),
    getRecent: jest.fn(() => Promise.resolve([])),
    postItem: jest.fn(() => Promise.resolve({})),
}));

const mocked = client as jest.Mocked<typeof client>;

const props = {
    channelId: 'channel1',
    rootId: 'root1',
    filter: '',
    onFilterChange: jest.fn(),
    onSelectDone: jest.fn(),
    isFusion: false,
};

beforeEach(async () => {
    jest.clearAllMocks();
    resetFavorites();
    mocked.featured.mockImplementation((kind) => Promise.resolve({items: kind === 'gif' ? [makeGif('g1')] : [makeSticker('s1')], next: ''}));
    mocked.search.mockImplementation((kind, q) => Promise.resolve({items: kind === 'gif' ? [makeGif('found', q)] : [makeSticker('found', q)]}));
    await loadConfig();
});

test('posts a GIF to the message box and closes the emoji picker', async () => {
    const {container} = renderWithProviders(<GifsTab {...props}/>);
    expect(container.firstChild).toHaveClass('amgifs-tab');
    expect(screen.getByLabelText('Search GIFs')).toHaveFocus();

    fireEvent.click(await screen.findByRole('button', {name: 'Send GIF: Happy cat'}));
    await waitFor(() => expect(props.onSelectDone).toHaveBeenCalled());
    expect(mocked.postItem).toHaveBeenCalledWith(expect.objectContaining({id: 'g1'}), 'channel1', 'root1', '');
});

test('shares the search box of the emoji picker', async () => {
    renderWithProviders(
        <GifsTab
            {...props}
            filter='party'
        />,
    );
    expect(screen.getByLabelText('Search GIFs')).toHaveValue('party');
    expect(await screen.findByRole('button', {name: 'Send GIF: party'})).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Search GIFs'), {target: {value: 'cats'}});
    expect(props.onFilterChange).toHaveBeenCalledWith('cats');
});

test('uses the Fusion markup in the Fusion picker', async () => {
    const {container} = renderWithProviders(
        <StickersTab
            {...props}
            isFusion={true}
        />,
    );
    expect(container.firstChild).toHaveClass('am-picker-search');
    expect(screen.getByLabelText('Search stickers')).toBeInTheDocument();
    expect(await screen.findByRole('button', {name: 'Send sticker: It works!'})).toHaveClass('am-sticker');
});

test('renders nothing outside of a message box', () => {
    const {container} = renderWithProviders(
        <GifsTab
            {...props}
            channelId=''
        />,
    );
    expect(container).toBeEmptyDOMElement();
});
