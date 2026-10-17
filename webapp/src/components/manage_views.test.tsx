// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {act, fireEvent, screen, waitFor, within} from '@testing-library/react';
import React from 'react';

import {fakeKit} from '../../tests/fake_kit';
import {makeGif, makeSticker, renderWithProviders} from '../../tests/utils';
import * as client from '../client';
import {loadConfig, loadFavorites, resetFavorites} from '../state';
import type {ClientConfig, UploadEntry} from '../types/gifs';

import {GifsTab, StickersTab} from './emoji_picker_tab';
import {isAcceptedFile, tagsProblem} from './manage_views';

jest.mock('../client', () => ({
    getConfig: jest.fn(),
    featured: jest.fn(() => Promise.resolve({items: [], next: ''})),
    search: jest.fn(() => Promise.resolve({items: []})),
    categories: jest.fn(() => Promise.resolve([{name: 'Lab crew', searchterm: 'lab crew'}])),
    autocomplete: jest.fn(() => Promise.resolve([])),
    getFavorites: jest.fn(() => Promise.resolve([])),
    addFavorite: jest.fn(),
    removeFavorite: jest.fn(),
    getRecent: jest.fn(() => Promise.resolve([])),
    postItem: jest.fn(() => Promise.resolve({})),
    upload: jest.fn(),
    listUploads: jest.fn(),
    deleteUpload: jest.fn(() => Promise.resolve()),
}));

const mocked = client as jest.Mocked<typeof client>;

const member: ClientConfig = {configured: true, stickers: true, can_upload: true, can_delete_others: false, upload_max_bytes: 16 << 20};

const props = {
    channelId: 'channel1',
    rootId: 'root1',
    filter: '',
    onFilterChange: jest.fn(),
    onSelectDone: jest.fn(),
    isFusion: true,
    pickerKit: fakeKit,
};

const entry = (id: string, title: string, mine: boolean, extra: Partial<UploadEntry> = {}): UploadEntry => ({
    id,
    kind: 'gif',
    title,
    user_id: mine ? 'user1' : 'user2',
    username: mine ? 'maya' : 'nina',
    created_at: Date.UTC(2026, 9, 1),
    item: makeGif(id, title),
    can_delete: mine,
    ...extra,
});

async function withConfig(config: ClientConfig) {
    mocked.getConfig.mockResolvedValue(config);
    await act(async () => {
        await loadConfig();
    });
}

function chooseFile(name: string, type: string, size = 10) {
    const file = new File(['x'.repeat(size)], name, {type});
    fireEvent.change(screen.getByLabelText('Choose a file, drop it here or paste it'), {target: {files: [file]}});
    return file;
}

beforeAll(() => {
    URL.createObjectURL = jest.fn(() => 'blob:preview');
    URL.revokeObjectURL = jest.fn();
});

beforeEach(async () => {
    jest.clearAllMocks();
    resetFavorites();
    mocked.listUploads.mockResolvedValue({uploads: [], has_more: false});
    await withConfig(member);
});

describe('checks', () => {
    test('file types as the GIF service takes them', () => {
        expect(isAcceptedFile('gif', new File([''], 'a.mp4', {type: 'video/mp4'}))).toBe(true);
        expect(isAcceptedFile('gif', new File([''], 'a.svg', {type: 'image/svg+xml'}))).toBe(false);
        expect(isAcceptedFile('sticker', new File([''], 'a.svg', {type: ''}))).toBe(true);
        expect(isAcceptedFile('sticker', new File([''], 'a.webm', {type: 'video/webm'}))).toBe(false);
    });

    test('tags as the GIF service counts them', () => {
        expect(tagsProblem('a, b, A ,')).toBe('');
        expect(tagsProblem(Array.from({length: 21}, (_, i) => 't' + i).join(','))).toBe('too_many');
        expect(tagsProblem('x'.repeat(41))).toBe('too_long');
    });
});

describe('buttons', () => {
    test('add and manage next to the search box, for people who may add', () => {
        renderWithProviders(<GifsTab {...props}/>);
        expect(screen.getByRole('button', {name: 'Add a GIF'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Manage GIFs'})).toBeInTheDocument();
    });

    test('none for people who may neither add nor delete', async () => {
        await withConfig({...member, can_upload: false, can_delete_others: false});
        renderWithProviders(<GifsTab {...props}/>);
        expect(screen.queryByRole('button', {name: 'Add a GIF'})).toBeNull();
        expect(screen.queryByRole('button', {name: 'Manage GIFs'})).toBeNull();
    });

    test('none for guests, even when the server would let everyone add', () => {
        const {unmount} = renderWithProviders(<GifsTab {...props}/>, 'system_guest');
        expect(screen.queryByRole('button', {name: 'Add a GIF'})).toBeNull();
        expect(screen.queryByRole('button', {name: 'Manage GIFs'})).toBeNull();
        unmount();

        renderWithProviders(<StickersTab {...props}/>, 'system_guest');
        expect(screen.queryByRole('button', {name: 'Add a sticker'})).toBeNull();
        expect(screen.queryByRole('button', {name: 'Manage stickers'})).toBeNull();
    });

    test('only manage for admins of a service that takes uploads from admins only', async () => {
        await withConfig({...member, can_upload: false, can_delete_others: true});
        renderWithProviders(<GifsTab {...props}/>);
        expect(screen.queryByRole('button', {name: 'Add a GIF'})).toBeNull();
        expect(screen.getByRole('button', {name: 'Manage GIFs'})).toBeInTheDocument();
    });

    test('none when GIFs aren’t set up, or stickers are off', async () => {
        await withConfig({...member, configured: false});
        const {unmount} = renderWithProviders(<GifsTab {...props}/>);
        expect(screen.queryByRole('button', {name: 'Add a GIF'})).toBeNull();
        expect(screen.queryByRole('button', {name: 'Manage GIFs'})).toBeNull();
        unmount();

        await withConfig({...member, stickers: false});
        renderWithProviders(<StickersTab {...props}/>);
        expect(screen.queryByRole('button', {name: 'Add a sticker'})).toBeNull();
        expect(screen.queryByRole('button', {name: 'Manage stickers'})).toBeNull();
    });

    test('without the host’s kit, the old button opening the upload dialog', () => {
        renderWithProviders(
            <GifsTab
                {...props}
                pickerKit={undefined}
            />,
        );
        expect(screen.getByRole('button', {name: 'Add a GIF'})).toHaveClass('am-icon-btn');
        expect(screen.queryByRole('button', {name: 'Manage GIFs'})).toBeNull();
    });
});

describe('add view', () => {
    test('checks the file and title, adds a sticker to a pack, and goes back to the grid with it first', async () => {
        mocked.upload.mockResolvedValue(makeSticker('new1', 'Lab dance'));
        renderWithProviders(<StickersTab {...props}/>);
        fireEvent.click(screen.getByRole('button', {name: 'Add a sticker'}));

        // In the picker: the tab's view, not a dialog, and the picker isn't closed.
        expect(screen.getByRole('heading', {name: 'Add a sticker'})).toBeInTheDocument();
        expect(screen.queryByRole('dialog')).toBeNull();
        expect(props.onSelectDone).not.toHaveBeenCalled();
        const save = screen.getByRole('button', {name: 'Add'});
        expect(save).toBeDisabled();
        expect(screen.getByText(/PNG, WebP, GIF or SVG, up to 16 MB/)).toBeInTheDocument();

        // A video isn't a sticker, nor a file over the limit.
        chooseFile('clip.mp4', 'video/mp4');
        expect(screen.getByRole('alert')).toHaveTextContent('can’t be a sticker');
        expect(screen.getByLabelText('Title')).toHaveValue('Clip');
        expect(save).toBeDisabled();
        chooseFile('huge.png', 'image/png', (16 << 20) + 1);
        expect(screen.getByRole('alert')).toHaveTextContent('larger than 16 MB');
        expect(save).toBeDisabled();

        // A good one shows its previews; the title is checked as it's typed.
        const file = chooseFile('lab_dance.png', 'image/png');
        expect(screen.queryByRole('alert')).toBeNull();
        fireEvent.change(screen.getByLabelText('Title'), {target: {value: ''}});
        fireEvent.blur(screen.getByLabelText('Title'));
        expect(screen.getByText('Give it a title: it helps people find it.')).toBeInTheDocument();
        expect(save).toBeDisabled();
        fireEvent.change(screen.getByLabelText('Title'), {target: {value: 'Lab dance'}});
        expect(screen.getByText('In the picker')).toBeInTheDocument();
        expect(screen.getByText('In a message')).toBeInTheDocument();

        fireEvent.change(screen.getByLabelText('Tags'), {target: {value: 'x'.repeat(41)}});
        expect(screen.getByText('A tag can have up to 40 characters.')).toBeInTheDocument();
        expect(save).toBeDisabled();
        fireEvent.change(screen.getByLabelText('Tags'), {target: {value: 'dance, lab'}});
        fireEvent.change(screen.getByLabelText('Sticker pack'), {target: {value: 'Lab crew'}});
        fireEvent.click(save);

        await waitFor(() => expect(mocked.upload).toHaveBeenCalledWith({kind: 'sticker', file, title: 'Lab dance', tags: 'dance, lab', pack: 'Lab crew'}));

        // Back to the grid: the new sticker first, with the focus, ready to send.
        const strip = await screen.findByRole('status');
        expect(strip).toHaveTextContent('Added: “Lab dance”');
        const send = within(strip).getByRole('button', {name: 'Send sticker: Lab dance'});
        expect(send).toHaveFocus();
        expect(screen.queryByRole('heading', {name: 'Add a sticker'})).toBeNull();
        expect(props.onSelectDone).not.toHaveBeenCalled();

        fireEvent.click(send);
        await waitFor(() => expect(mocked.postItem).toHaveBeenCalledWith(expect.objectContaining({id: 'new1'}), 'channel1', 'root1', ''));
    });

    test('shows the server’s reason in place, and Back returns to the grid', async () => {
        mocked.upload.mockRejectedValue(new Error('The file couldn’t be converted'));
        renderWithProviders(<GifsTab {...props}/>);
        fireEvent.click(screen.getByRole('button', {name: 'Add a GIF'}));
        expect(screen.queryByLabelText('Sticker pack')).toBeNull();

        chooseFile('wave.gif', 'image/gif');
        fireEvent.click(screen.getByRole('button', {name: 'Add'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('Couldn’t add it: The file couldn’t be converted');
        expect(screen.getByRole('heading', {name: 'Add a GIF'})).toBeInTheDocument();

        fireEvent.click(screen.getByRole('button', {name: 'Back'}));
        expect(screen.getByLabelText('Search GIFs')).toBeInTheDocument();
        expect(screen.queryByRole('heading', {name: 'Add a GIF'})).toBeNull();
        expect(props.onSelectDone).not.toHaveBeenCalled();
    });
});

describe('manage view', () => {
    test('lists the uploads with who added them, searches, and pages', async () => {
        mocked.listUploads.mockImplementation((kind, q, page) => Promise.resolve(q ? {uploads: [entry('g3', 'Wave', false)], has_more: false} : {uploads: page ? [entry('g2', 'Older', true)] : [entry('g1', 'Lab wave', true)], has_more: !page}));
        renderWithProviders(<GifsTab {...props}/>);
        fireEvent.click(screen.getByRole('button', {name: 'Manage GIFs'}));

        expect(await screen.findByText('Lab wave')).toBeInTheDocument();
        expect(screen.getByText('@maya')).toBeInTheDocument();
        expect(mocked.listUploads).toHaveBeenCalledWith('gif', '', 0, 30);
        fireEvent.click(screen.getByRole('button', {name: 'Show more'}));
        expect(await screen.findByText('Older')).toBeInTheDocument();
        expect(mocked.listUploads).toHaveBeenCalledWith('gif', '', 1, 30);

        fireEvent.change(screen.getByLabelText('Search added GIFs'), {target: {value: 'wave'}});
        expect(await screen.findByText('Wave')).toBeInTheDocument();
        expect(mocked.listUploads).toHaveBeenCalledWith('gif', 'wave', 0, 30);
        expect(screen.queryByText('Lab wave')).toBeNull();
    });

    test('deletes only what the person may, after asking in the row', async () => {
        mocked.listUploads.mockResolvedValue({uploads: [entry('g1', 'Mine', true), entry('g2', 'Theirs', false)], has_more: false});
        mocked.getFavorites.mockResolvedValue([makeGif('g1', 'Mine')]);
        await act(async () => {
            await loadFavorites();
        });
        renderWithProviders(<GifsTab {...props}/>);
        fireEvent.click(screen.getByRole('button', {name: 'Manage GIFs'}));
        await screen.findByText('Mine');

        expect(screen.getByRole('button', {name: 'Delete “Mine”'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Delete “Theirs”'})).toBeNull();

        // Keep: nothing deleted.
        fireEvent.click(screen.getByRole('button', {name: 'Delete “Mine”'}));
        expect(screen.getByText(/Delete “Mine” for everyone\?/)).toBeInTheDocument();
        fireEvent.click(screen.getByRole('button', {name: 'Keep'}));
        expect(mocked.deleteUpload).not.toHaveBeenCalled();

        fireEvent.click(screen.getByRole('button', {name: 'Delete “Mine”'}));
        fireEvent.click(screen.getByRole('button', {name: 'Delete'}));
        await waitFor(() => expect(mocked.deleteUpload).toHaveBeenCalledWith('gif', 'g1'));
        expect(await screen.findByRole('status')).toHaveTextContent('“Mine” is deleted.');
        expect(screen.queryByText('Mine')).toBeNull();
        expect(screen.getByText('Theirs')).toBeInTheDocument();
        expect(props.onSelectDone).not.toHaveBeenCalled();
    });

    test('admins may delete anyone’s; the server’s refusal shows in place', async () => {
        await withConfig({...member, can_delete_others: true});
        mocked.listUploads.mockResolvedValue({uploads: [entry('s2', 'Theirs', false, {kind: 'sticker', can_delete: true, item: makeSticker('s2', 'Theirs')})], has_more: false});
        mocked.deleteUpload.mockRejectedValueOnce(new Error('you can’t delete this upload'));
        renderWithProviders(<StickersTab {...props}/>);
        fireEvent.click(screen.getByRole('button', {name: 'Manage stickers'}));

        fireEvent.click(await screen.findByRole('button', {name: 'Delete “Theirs”'}));
        expect(screen.getByText(/It leaves its sticker pack/)).toBeInTheDocument();
        fireEvent.click(screen.getByRole('button', {name: 'Delete'}));
        await waitFor(() => expect(mocked.deleteUpload).toHaveBeenCalledWith('sticker', 's2'));
        expect(await screen.findByRole('status')).toHaveTextContent('Couldn’t delete it: you can’t delete this upload');
        expect(screen.getByText('Theirs')).toBeInTheDocument();
    });

    test('says so when nothing was added, and Back returns to the grid', async () => {
        renderWithProviders(<GifsTab {...props}/>);
        fireEvent.click(screen.getByRole('button', {name: 'Manage GIFs'}));
        expect(await screen.findByText('Nobody has added a GIF yet.')).toBeInTheDocument();
        fireEvent.click(screen.getByRole('button', {name: 'Back'}));
        expect(screen.getByLabelText('Search GIFs')).toBeInTheDocument();
    });
});
