// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {act, fireEvent, screen, waitFor} from '@testing-library/react';
import React from 'react';

import {makeSticker, renderWithProviders} from '../../tests/utils';
import * as client from '../client';
import {closeUpload, loadConfig, openUpload, resetFavorites} from '../state';

import {GifsTab} from './emoji_picker_tab';
import UploadDialogRoot, {titleFromFile} from './upload_dialog';

jest.mock('../client', () => ({
    getConfig: jest.fn(() => Promise.resolve({configured: true, stickers: true, can_upload: true})),
    featured: jest.fn(() => Promise.resolve({items: [], next: ''})),
    search: jest.fn(() => Promise.resolve({items: []})),
    categories: jest.fn(() => Promise.resolve([{name: 'Lab crew', searchterm: 'lab crew'}])),
    autocomplete: jest.fn(() => Promise.resolve([])),
    getFavorites: jest.fn(() => Promise.resolve([])),
    getRecent: jest.fn(() => Promise.resolve([])),
    upload: jest.fn(),
    postItem: jest.fn(() => Promise.resolve({})),
}));

const mocked = client as jest.Mocked<typeof client>;

function chooseFile(name: string, size = 10) {
    const file = new File(['x'.repeat(size)], name, {type: 'image/png'});
    fireEvent.change(screen.getByLabelText('File'), {target: {files: [file]}});
    return file;
}

beforeEach(async () => {
    jest.clearAllMocks();
    resetFavorites();
    delete window.antimatterWebUI;
    act(() => closeUpload());
    mocked.getConfig.mockResolvedValue({configured: true, stickers: true, can_upload: true});
    await loadConfig();
});

test('titleFromFile', () => {
    expect(titleFromFile('happy_cat-dance.gif')).toBe('Happy cat dance');
    expect(titleFromFile('.gif')).toBe('');
});

test('the picker panel opens the dialog when the user can add GIFs', async () => {
    const onSelectDone = jest.fn();
    renderWithProviders(
        <>
            <GifsTab
                channelId='channel1'
                filter=''
                onFilterChange={jest.fn()}
                onSelectDone={onSelectDone}
                isFusion={false}
            />
            <UploadDialogRoot/>
        </>,
    );
    fireEvent.click(screen.getByRole('button', {name: 'Add a GIF'}));
    expect(onSelectDone).toHaveBeenCalled();
    expect(screen.getByRole('dialog', {name: 'Add a GIF'})).toHaveClass('amgifs-modal');
});

test('the picker panel has no add button for other users', async () => {
    mocked.getConfig.mockResolvedValue({configured: true, stickers: true, can_upload: false});
    await loadConfig();
    renderWithProviders(
        <GifsTab
            channelId='channel1'
            filter=''
            onFilterChange={jest.fn()}
            onSelectDone={jest.fn()}
            isFusion={false}
        />,
    );
    expect(screen.queryByRole('button', {name: 'Add a GIF'})).toBeNull();
});

test('adds a sticker to a pack and sends it', async () => {
    mocked.upload.mockResolvedValue(makeSticker('new1', 'Lab dance'));
    renderWithProviders(<UploadDialogRoot/>);
    act(() => openUpload({kind: 'sticker', channelId: 'channel1', rootId: 'root1'}));

    const add = screen.getByRole('button', {name: 'Add'});
    expect(add).toBeDisabled();
    const file = chooseFile('lab_dance.png');
    expect(screen.getByLabelText('Title')).toHaveValue('Lab dance');
    fireEvent.change(screen.getByLabelText('Tags'), {target: {value: 'dance, lab'}});
    fireEvent.change(screen.getByLabelText('Sticker pack'), {target: {value: 'Lab crew'}});
    fireEvent.click(add);

    expect(await screen.findByText('Added! It’s in your favorites.')).toBeInTheDocument();
    expect(mocked.upload).toHaveBeenCalledWith({kind: 'sticker', file, title: 'Lab dance', tags: 'dance, lab', pack: 'Lab crew'});

    fireEvent.click(screen.getByRole('button', {name: 'Send it'}));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(mocked.postItem).toHaveBeenCalledWith(expect.objectContaining({id: 'new1'}), 'channel1', 'root1');
});

test('shows why a GIF can’t be added', async () => {
    mocked.upload.mockRejectedValue(new Error('The file couldn’t be converted'));
    renderWithProviders(<UploadDialogRoot/>);
    act(() => openUpload({kind: 'gif'}));
    expect(screen.queryByLabelText('Sticker pack')).toBeNull();

    chooseFile('big.gif', (16 << 20) + 1);
    expect(screen.getByRole('alert')).toHaveTextContent('larger than 16 MB');
    expect(screen.getByRole('button', {name: 'Add'})).toBeDisabled();

    chooseFile('small.gif');
    fireEvent.click(screen.getByRole('button', {name: 'Add'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('Couldn’t add it: The file couldn’t be converted');

    fireEvent.keyDown(document, {key: 'Escape'});
    expect(screen.queryByRole('dialog')).toBeNull();
});

test('uses the Fusion dialog markup in its layer', () => {
    window.antimatterWebUI = 'fusion';
    renderWithProviders(<UploadDialogRoot/>);
    act(() => openUpload({kind: 'gif', channelId: 'channel1'}));
    const dialog = screen.getByRole('dialog', {name: 'Add a GIF'});
    expect(dialog).toHaveClass('am-modal', 'am-small', 'amgifs-upload');
    expect(dialog.parentElement).toHaveClass('am-scrim');
    expect(dialog.parentElement?.parentElement).toHaveClass('am-layer');
});

test('under Fusion, the file field is a Fusion button over the browser\'s input', () => {
    window.antimatterWebUI = 'fusion';
    renderWithProviders(<UploadDialogRoot/>);
    act(() => openUpload({kind: 'gif', channelId: 'channel1'}));

    // The input is still the field's control, out of sight behind the button.
    const input = screen.getByLabelText('File');
    expect(input).toHaveAttribute('type', 'file');
    expect(input).toHaveClass('amgifs-file-input');
    const button = input.parentElement!.querySelector('.amgifs-file-btn')!;
    expect(button).toHaveClass('am-btn');
    expect(button).toHaveAttribute('aria-hidden', 'true');
    expect(button).toHaveTextContent('Choose a file');
    expect(button.querySelector('use')).toHaveAttribute('href', '#am-i-upload');
    expect(input.closest('label')).toHaveClass('am-field');

    chooseFile('happy_cat.gif');
    expect(button).toHaveTextContent('Choose another file');
    expect(screen.getByText('happy_cat.gif')).toHaveClass('amgifs-file-name');
    expect(screen.getByLabelText('Title')).toHaveValue('Happy cat');
});

test('under the classic UI, the file field is the browser\'s input, as before', () => {
    renderWithProviders(<UploadDialogRoot/>);
    act(() => openUpload({kind: 'gif', channelId: 'channel1'}));
    const input = screen.getByLabelText('File');
    expect(input).not.toHaveClass('amgifs-file-input');
    expect(document.querySelector('.amgifs-file-btn')).toBeNull();
});

test('the add button has Fusion\'s tooltip under Fusion', async () => {
    window.antimatterWebUI = 'fusion';
    renderWithProviders(
        <GifsTab
            channelId='channel1'
            filter=''
            onFilterChange={jest.fn()}
            onSelectDone={jest.fn()}
            isFusion={true}
        />,
    );
    const add = await screen.findByRole('button', {name: 'Add a GIF'});
    expect(add).toHaveAttribute('data-am-tip', 'Add a GIF');
    expect(add).not.toHaveAttribute('title');
    expect(add.querySelector('use')).toHaveAttribute('href', '#am-i-plus');
});
