// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';

import * as client from '../client';
import {messages} from '../messages';
import {addedFavorite, removedFavorite, useClientConfig} from '../state';
import type {ClientConfig, Item, Kind, UploadEntry} from '../types/gifs';
import type {PickerKit, PickerListItem} from '../types/host';

import {useDebounced, useLoad} from './hooks';
import MediaView from './media_view';
import {PickerPanel} from './picker';
import {titleFromFile} from './upload_dialog';

// The largest file, unless the server says otherwise (as it does).
const DEFAULT_MAX_BYTES = 16 << 20;

// The file types the GIF service takes, by kind (gifs-server's uploadTypes): MIME types and
// extensions, since browsers don't always know the type of a dropped or pasted file.
export const UPLOAD_TYPES: Record<Kind, {mime: string[]; ext: string[]}> = {
    gif: {mime: ['image/gif', 'image/png', 'image/apng', 'image/webp', 'video/mp4', 'video/webm'], ext: ['gif', 'png', 'apng', 'webp', 'mp4', 'webm']},
    sticker: {mime: ['image/png', 'image/webp', 'image/gif', 'image/svg+xml'], ext: ['png', 'webp', 'gif', 'svg']},
};

const accept = (kind: Kind) => [...UPLOAD_TYPES[kind].mime, ...UPLOAD_TYPES[kind].ext.map((e) => '.' + e)].join(',');

// isAcceptedFile tells whether the GIF service can take a file as a GIF or sticker.
export function isAcceptedFile(kind: Kind, file: File) {
    const ext = (/\.([a-z0-9]+)$/i).exec(file.name)?.[1]?.toLowerCase() || '';
    return UPLOAD_TYPES[kind].mime.includes(file.type) || UPLOAD_TYPES[kind].ext.includes(ext);
}

// tagsProblem checks the comma-separated tags as the GIF service does (up to 20, of 40 characters).
export function tagsProblem(tags: string): 'too_many' | 'too_long' | '' {
    const list = Array.from(new Set(tags.split(',').map((t) => t.trim().toLowerCase().split(/\s+/).join(' ')).filter(Boolean)));
    if (list.some((t) => Array.from(t).length > 40)) {
        return 'too_long';
    }
    return list.length > 20 ? 'too_many' : '';
}

const maxBytesOf = (config: ClientConfig | null) => config?.upload_max_bytes || DEFAULT_MAX_BYTES;

// canManage tells whether the manage view is of use to the person: they may add, or delete others'.
export function canManage(config: ClientConfig | null, kind: Kind) {
    return Boolean(config?.configured && (kind !== 'sticker' || config.stickers) && (config.can_upload || config.can_delete_others));
}

type PanelProps = {
    kit: PickerKit;
    kind: Kind;
    channelId: string;
    rootId?: string;
    query: string;
    onQuery: (query: string) => void;
    onPosted: () => void;
    onClose: () => void;
};

// PanelWithViews is the GIFs or Stickers tab of the host's Fusion picker with its add and manage
// views: the "+" and manage buttons next to the search box swap the grid for a view inside the
// picker, which stays open, and the view's back arrow (or Escape) returns to the grid.
export function PanelWithViews({kit, kind, channelId, rootId, query, onQuery, onPosted, onClose}: PanelProps) {
    const intl = useIntl();
    const config = useClientConfig();
    const [view, open, back] = kit.usePickerView<'add' | 'manage'>();
    const [added, setAdded] = useState<Item | null>(null);
    const sticker = kind === 'sticker';

    if (view === 'add') {
        return (
            <AddView
                kit={kit}
                kind={kind}
                config={config}
                onBack={() => back()}
                onAdded={(item) => {
                    setAdded(item);

                    // The grid puts the focus on what was added.
                    back(false);
                }}
            />
        );
    }
    if (view === 'manage') {
        return (
            <ManageView
                kit={kit}
                kind={kind}
                onBack={() => back()}
            />
        );
    }

    const allowed = Boolean(config?.configured && (!sticker || config.stickers));
    return (
        <PickerPanel
            kind={kind}
            fusion={true}
            channelId={channelId}
            rootId={rootId}
            query={query}
            onQuery={onQuery}
            onPosted={onPosted}
            onClose={onClose}
            added={added}
            onDismissAdded={() => setAdded(null)}
            tools={(
                <kit.PickerViewButtons
                    onAdd={allowed && config?.can_upload ? () => open('add') : undefined}
                    onManage={canManage(config, kind) ? () => open('manage') : undefined}
                    addLabel={intl.formatMessage(sticker ? messages.addSticker : messages.addGif)}
                    manageLabel={intl.formatMessage(sticker ? messages.manageStickers : messages.manageGifs)}
                />
            )}
        />
    );
}

// FilePreview shows the chosen file at the sizes it shows at: a cell of the picker and a message.
function FilePreview({file, kind}: {file: File; kind: Kind}) {
    const intl = useIntl();
    const url = useMemo(() => URL.createObjectURL(file), [file]);
    useEffect(() => () => URL.revokeObjectURL(url), [url]);
    const video = file.type.startsWith('video/') || (/\.(mp4|webm)$/i).test(file.name);
    const media = (className: string) => (video ? (
        <video
            className={className}
            src={url}
            autoPlay={true}
            loop={true}
            muted={true}
            playsInline={true}
        />
    ) : (
        <img
            className={className}
            src={url}
            alt=''
        />
    ));
    return (
        <div className={`amgifs-add-preview amgifs-add-preview-${kind}`}>
            <figure>
                {media('amgifs-add-preview-picker')}
                <figcaption>{intl.formatMessage(messages.addPreviewPicker)}</figcaption>
            </figure>
            <figure>
                {media('amgifs-add-preview-message')}
                <figcaption>{intl.formatMessage(messages.addPreviewMessage)}</figcaption>
            </figure>
        </div>
    );
}

type AddProps = {
    kit: PickerKit;
    kind: Kind;
    config: ClientConfig | null;
    onBack: () => void;
    onAdded: (item: Item) => void;
};

// AddView adds a GIF or sticker inside the picker: a file chosen, dropped or pasted, its title,
// tags and (stickers) pack, checked as the GIF service checks them, with the errors in place.
export function AddView({kit, kind, config, onBack, onAdded}: AddProps) {
    const intl = useIntl();
    const sticker = kind === 'sticker';
    const maxBytes = maxBytesOf(config);
    const sizeMB = Math.round(maxBytes / (1 << 20));
    const [file, setFile] = useState<File | null>(null);
    const [title, setTitle] = useState('');
    const [tags, setTags] = useState('');
    const [pack, setPack] = useState('');
    const [touched, setTouched] = useState(false);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const packs = useLoad(sticker ? 'packs' : null, () => client.categories('sticker'));

    let fileError = '';
    if (file && !isAcceptedFile(kind, file)) {
        fileError = intl.formatMessage(sticker ? messages.addWrongTypeSticker : messages.addWrongType);
    } else if (file && file.size > maxBytes) {
        fileError = intl.formatMessage(messages.addTooLarge, {size: sizeMB});
    }
    const cleanTitle = title.trim();
    let titleError = '';
    if (Array.from(cleanTitle).length > 100) {
        titleError = intl.formatMessage(messages.addTitleTooLong);
    } else if (!cleanTitle && touched) {
        titleError = intl.formatMessage(messages.addTitleRequired);
    }
    const tagsWhy = tagsProblem(tags);
    let tagsError = '';
    if (tagsWhy === 'too_many') {
        tagsError = intl.formatMessage(messages.addTagsTooMany);
    } else if (tagsWhy === 'too_long') {
        tagsError = intl.formatMessage(messages.addTagTooLong);
    }
    const packError = Array.from(pack.trim()).length > 60 ? intl.formatMessage(messages.addPackTooLong) : '';
    const valid = Boolean(file && cleanTitle && !fileError && !titleError && !tagsError && !packError);

    const onFile = (f: File) => {
        setError('');
        setFile(f);
        if (!title.trim()) {
            setTitle(titleFromFile(f.name));
        }
    };

    const save = async () => {
        setTouched(true);
        if (!valid || !file || busy) {
            return;
        }
        setBusy(true);
        setError('');
        try {
            const item = await client.upload({kind, file, title: cleanTitle, tags, pack: sticker ? pack.trim() : ''});
            addedFavorite(item);
            onAdded(item);
        } catch (err) {
            setBusy(false);
            setError(intl.formatMessage(messages.uploadFailed, {error: (err as Error).message}));
        }
    };

    return (
        <kit.PickerViewFrame
            title={intl.formatMessage(sticker ? messages.addSticker : messages.addGif)}
            onBack={onBack}
            onSubmit={save}
            actions={(
                <>
                    <button
                        type='button'
                        className='am-btn'
                        onClick={onBack}
                    >
                        {intl.formatMessage(messages.addCancel)}
                    </button>
                    <button
                        type='submit'
                        className='am-btn am-primary'
                        disabled={busy || !valid}
                    >
                        {intl.formatMessage(busy ? messages.uploading : messages.addSave)}
                    </button>
                </>
            )}
        >
            <kit.PickerDropZone
                accept={accept(kind)}
                file={file}
                onFile={onFile}
                label={intl.formatMessage(messages.addDrop)}
                changeLabel={intl.formatMessage(messages.addChange)}
                hint={intl.formatMessage(sticker ? messages.addStickerHint : messages.addGifHint, {size: sizeMB})}
                error={fileError || undefined}
                preview={file && !fileError ? (
                    <FilePreview
                        file={file}
                        kind={kind}
                    />
                ) : undefined}
            />
            <kit.PickerField
                label={intl.formatMessage(messages.uploadTitle)}
                error={titleError || undefined}
                aside={`${Array.from(cleanTitle).length}/100`}
            >
                {({id, describedBy, invalid}) => (
                    <input
                        id={id}
                        type='text'
                        value={title}
                        aria-describedby={describedBy}
                        aria-invalid={invalid}
                        aria-required={true}
                        autoComplete='off'
                        onChange={(e) => {
                            setTitle(e.target.value);
                            setError('');
                        }}
                        onBlur={() => setTouched(true)}
                    />
                )}
            </kit.PickerField>
            <kit.PickerField
                label={intl.formatMessage(messages.uploadTags)}
                hint={intl.formatMessage(messages.uploadTagsHint)}
                error={tagsError || undefined}
            >
                {({id, describedBy, invalid}) => (
                    <input
                        id={id}
                        type='text'
                        value={tags}
                        maxLength={500}
                        aria-describedby={describedBy}
                        aria-invalid={invalid}
                        autoComplete='off'
                        onChange={(e) => setTags(e.target.value)}
                    />
                )}
            </kit.PickerField>
            {sticker && (
                <kit.PickerField
                    label={intl.formatMessage(messages.uploadPack)}
                    hint={intl.formatMessage(messages.uploadPackHint)}
                    error={packError || undefined}
                >
                    {({id, describedBy, invalid}) => (
                        <>
                            <input
                                id={id}
                                type='text'
                                value={pack}
                                list='amgifs-add-packs'
                                aria-describedby={describedBy}
                                aria-invalid={invalid}
                                autoComplete='off'
                                onChange={(e) => setPack(e.target.value)}
                            />
                            <datalist id='amgifs-add-packs'>
                                {(packs.data || []).map((p) => (
                                    <option
                                        key={p.searchterm}
                                        value={p.name}
                                    />
                                ))}
                            </datalist>
                        </>
                    )}
                </kit.PickerField>
            )}
            <kit.PickerNotice>{error}</kit.PickerNotice>
        </kit.PickerViewFrame>
    );
}

const PER_PAGE = 30;

type ManageProps = {
    kit: PickerKit;
    kind: Kind;
    onBack: () => void;
};

// ManageView lists the GIFs or stickers people added, newest first, searchable, with who added
// them and when. People delete their own, system admins anyone's; the server checks it again.
export function ManageView({kit, kind, onBack}: ManageProps) {
    const intl = useIntl();
    const sticker = kind === 'sticker';
    const [query, setQuery] = useState('');
    const term = useDebounced(query.trim(), 250);
    const [entries, setEntries] = useState<UploadEntry[]>([]);
    const [page, setPage] = useState(0);
    const [hasMore, setHasMore] = useState(false);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');
    const termRef = useRef(term);
    termRef.current = term;

    const load = useCallback(async (forTerm: string, forPage: number) => {
        setLoading(true);
        setError('');
        try {
            const list = await client.listUploads(kind, forTerm, forPage, PER_PAGE);
            if (termRef.current !== forTerm) {
                return;
            }
            setEntries((prev) => (forPage === 0 ? list.uploads : [...prev, ...list.uploads.filter((u) => !prev.some((p) => p.id === u.id))]));
            setHasMore(list.has_more);
            setPage(forPage);
        } catch (err) {
            if (termRef.current === forTerm) {
                setError(intl.formatMessage(messages.manageLoadFailed, {error: (err as Error).message}));
                setHasMore(false);
            }
        } finally {
            if (termRef.current === forTerm) {
                setLoading(false);
            }
        }
    }, [kind, intl]);

    useEffect(() => {
        load(term, 0);
    }, [term, load]);

    const untitled = intl.formatMessage(messages.manageUntitled);
    const items: PickerListItem[] = entries.map((e) => ({
        id: e.id,
        name: e.title || untitled,
        image: e.item ? (
            <MediaView
                media={e.item.preview}
                label=''
                className='amgifs-contain'
            />
        ) : null,
        by: e.username ? '@' + e.username : undefined,
        at: e.created_at || undefined,
        canDelete: e.can_delete,
    }));

    const onDelete = async (item: PickerListItem) => {
        try {
            await client.deleteUpload(kind, item.id);
        } catch (err) {
            return intl.formatMessage(messages.manageDeleteFailed, {error: (err as Error).message});
        }
        setEntries((prev) => prev.filter((e) => e.id !== item.id));
        removedFavorite(kind, item.id);
        return undefined;
    };

    let empty = intl.formatMessage(sticker ? messages.manageEmptyStickers : messages.manageEmpty);
    if (term) {
        empty = intl.formatMessage(messages.manageNoMatch);
    }

    return (
        <kit.PickerViewFrame
            title={intl.formatMessage(sticker ? messages.manageStickers : messages.manageGifs)}
            onBack={onBack}
        >
            <kit.PickerSearch
                value={query}
                onChange={setQuery}
                placeholder={intl.formatMessage(sticker ? messages.manageSearchStickers : messages.manageSearch)}
            />
            <kit.PickerNotice>{error}</kit.PickerNotice>
            <kit.PickerManageList
                items={items}
                onDelete={onDelete}
                deleteLabel={(item) => intl.formatMessage(messages.manageDelete, {title: item.name})}
                confirmText={(item) => intl.formatMessage(sticker ? messages.manageConfirmSticker : messages.manageConfirmGif, {title: item.name})}
                deletedText={(item) => intl.formatMessage(messages.manageDeleted, {title: item.name})}
                empty={error ? '' : empty}
                loading={loading}
                hasMore={hasMore}
                onLoadMore={() => load(term, page + 1)}
            />
        </kit.PickerViewFrame>
    );
}
