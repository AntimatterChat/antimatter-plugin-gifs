// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import ReactDOM from 'react-dom';
import {useIntl} from 'react-intl';

import * as client from '../client';
import {messages} from '../messages';
import {addedFavorite, closeUpload, openUpload, useUpload, type UploadRequest} from '../state';
import type {Item} from '../types/gifs';
import {classNames, isFusionUI} from '../web_ui';

import {iconTip} from './fusion/tip';
import {useLoad} from './hooks';
import {PlusIcon, UploadIcon} from './icons';
import MediaView from './media_view';
import {layerContainer} from './picker_root';

// The largest file, as on the server.
const MAX_BYTES = 16 << 20;

const ACCEPT = {
    gif: 'image/gif,image/png,image/apng,image/webp,video/mp4,video/webm,.gif,.png,.apng,.webp,.mp4,.webm',
    sticker: 'image/png,image/webp,image/gif,image/svg+xml,.png,.webp,.gif,.svg',
};

// titleFromFile makes a title of a file name: "happy_cat-dance.gif" is "Happy cat dance".
export function titleFromFile(name: string) {
    const words = name.replace(/\.[^.]+$/, '').split(/[-_.\s]+/).filter(Boolean).join(' ');
    return words.charAt(0).toUpperCase() + words.slice(1);
}

type Props = {
    request: UploadRequest;
    fusion: boolean;
};

// UploadDialog adds a GIF or sticker to the GIF service, then offers to send it where the picker
// was opened. It has the markup of the Fusion UI mockup's dialogs (am-scrim, am-modal...) under
// Fusion, the plugin's amgifs- classes under the classic UI.
export function UploadDialog({request, fusion}: Props) {
    const intl = useIntl();
    const k = classNames(fusion);
    const sticker = request.kind === 'sticker';
    const [file, setFile] = useState<File | null>(null);
    const [title, setTitle] = useState('');
    const [tags, setTags] = useState('');
    const [pack, setPack] = useState('');
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [added, setAdded] = useState<Item | null>(null);
    const packs = useLoad(sticker ? 'packs' : null, () => client.categories('sticker'));

    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.stopPropagation();
                closeUpload();
            }
        };
        document.addEventListener('keydown', onKeyDown, true);
        return () => document.removeEventListener('keydown', onKeyDown, true);
    }, []);

    const heading = intl.formatMessage(sticker ? messages.addSticker : messages.addGif);

    const onFile = (f: File | null) => {
        setError('');
        setFile(f);
        if (f && f.size > MAX_BYTES) {
            setError(intl.formatMessage(messages.uploadTooLarge));
        }
        if (f && !title.trim()) {
            setTitle(titleFromFile(f.name));
        }
    };

    const onSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!file || !title.trim() || file.size > MAX_BYTES || busy) {
            return;
        }
        setBusy(true);
        setError('');
        try {
            const item = await client.upload({kind: request.kind, file, title: title.trim(), tags, pack: sticker ? pack.trim() : ''});
            addedFavorite(item);
            setAdded(item);
        } catch (err) {
            setError(intl.formatMessage(messages.uploadFailed, {error: (err as Error).message}));
        } finally {
            setBusy(false);
        }
    };

    const onSend = async () => {
        if (!added || !request.channelId) {
            return;
        }
        setBusy(true);
        try {
            await client.postItem(added, request.channelId, request.rootId);
            closeUpload();
        } catch (err) {
            setBusy(false);
            setError(intl.formatMessage(messages.postFailed, {error: (err as Error).message}));
        }
    };

    // A field of the mockup's dialogs: its label, its control and an optional hint describing it.
    type A11y = {'aria-label': string; 'aria-describedby'?: string};
    const field = (name: string, label: string, control: (a11y: A11y) => React.ReactNode, hint?: string) => (
        <label className={k('field')}>
            <span>{label}</span>
            {control(hint ? {'aria-label': label, 'aria-describedby': `amgifs-upload-${name}-hint`} : {'aria-label': label})}
            {hint && <small id={`amgifs-upload-${name}-hint`}>{hint}</small>}
        </label>
    );

    const errorBox = error && (
        <div
            className='amgifs-error amgifs-upload-error'
            role='alert'
        >
            {error}
        </div>
    );

    let body;
    if (added) {
        body = (
            <>
                <div className='amgifs-upload-added'>
                    <MediaView
                        media={added.preview}
                        label={added.title || ''}
                        className='amgifs-contain'
                    />
                </div>
                <p className={k('lead')}>{intl.formatMessage(messages.uploadDone)}</p>
                {errorBox}
                <div className={k('modal-actions')}>
                    <button
                        type='button'
                        className={k('btn')}
                        onClick={closeUpload}
                    >
                        {intl.formatMessage(messages.uploadClose)}
                    </button>
                    {request.channelId && (
                        <button
                            type='button'
                            className={k('btn', 'primary')}
                            disabled={busy}
                            onClick={onSend}
                        >
                            {intl.formatMessage(messages.uploadSend)}
                        </button>
                    )}
                </div>
            </>
        );
    } else {
        body = (
            <>
                <p className={k('lead')}>{intl.formatMessage(sticker ? messages.uploadStickerLead : messages.uploadGifLead)}</p>
                {field('file', intl.formatMessage(messages.uploadFile), (a11y) => (fusion ? (
                    <FusionFileInput
                        a11y={a11y}
                        accept={ACCEPT[request.kind]}
                        file={file}
                        onFile={onFile}
                    />
                ) : (
                    <input
                        {...a11y}
                        type='file'
                        accept={ACCEPT[request.kind]}
                        onChange={(e) => onFile(e.target.files?.[0] || null)}
                    />
                )))}
                {field('title', intl.formatMessage(messages.uploadTitle), (a11y) => (
                    <input
                        {...a11y}
                        type='text'
                        value={title}
                        maxLength={100}
                        required={true}
                        onChange={(e) => setTitle(e.target.value)}
                    />
                ))}
                {field('tags', intl.formatMessage(messages.uploadTags), (a11y) => (
                    <input
                        {...a11y}
                        type='text'
                        value={tags}
                        maxLength={500}
                        onChange={(e) => setTags(e.target.value)}
                    />
                ), intl.formatMessage(messages.uploadTagsHint))}
                {sticker && field('pack', intl.formatMessage(messages.uploadPack), (a11y) => (
                    <>
                        <input
                            {...a11y}
                            type='text'
                            value={pack}
                            maxLength={60}
                            list='amgifs-upload-packs'
                            onChange={(e) => setPack(e.target.value)}
                        />
                        <datalist id='amgifs-upload-packs'>
                            {(packs.data || []).map((p) => (
                                <option
                                    key={p.searchterm}
                                    value={p.name}
                                />
                            ))}
                        </datalist>
                    </>
                ), intl.formatMessage(messages.uploadPackHint))}
                {errorBox}
                <div className={k('modal-actions')}>
                    <button
                        type='button'
                        className={k('btn')}
                        onClick={closeUpload}
                    >
                        {intl.formatMessage(messages.uploadCancel)}
                    </button>
                    <button
                        type='submit'
                        className={k('btn', 'primary')}
                        disabled={busy || !file || !title.trim() || file.size > MAX_BYTES}
                    >
                        {intl.formatMessage(busy ? messages.uploading : messages.uploadSubmit)}
                    </button>
                </div>
            </>
        );
    }

    return (
        <div
            className={k('scrim')}
            onMouseDown={(e) => {
                if (e.target === e.currentTarget && !busy) {
                    closeUpload();
                }
            }}
        >
            <form
                className={k('modal', 'small') + ' amgifs-upload'}
                role='dialog'
                aria-modal='true'
                aria-label={heading}
                onSubmit={onSubmit}
            >
                <div className={k('pane')}>
                    <h2>{heading}</h2>
                    {body}
                </div>
            </form>
        </div>
    );
}

type FileInputProps = {
    a11y: {'aria-label': string; 'aria-describedby'?: string};
    accept: string;
    file: File | null;
    onFile: (file: File | null) => void;
};

// FusionFileInput is the dialog's file field under Fusion: the browser's file input, out of sight
// but still the control the keyboard and screen readers reach, behind a Fusion button that shows
// the chosen file. The field's label around it opens the file chooser on a click.
export function FusionFileInput({a11y, accept, file, onFile}: FileInputProps) {
    const intl = useIntl();
    return (
        <div className='amgifs-file'>
            <input
                {...a11y}
                type='file'
                className='amgifs-file-input'
                accept={accept}
                onChange={(e) => onFile(e.target.files?.[0] || null)}
            />
            <span
                className='am-btn amgifs-file-btn'
                aria-hidden='true'
            >
                <UploadIcon className='am-ic am-sm'/>
                {intl.formatMessage(file ? messages.uploadChange : messages.uploadChoose)}
            </span>
            {file && <span className='amgifs-file-name'>{file.name}</span>}
        </div>
    );
}

// AddButton opens the dialog adding a GIF or sticker, after closing the picker it's in.
export function AddButton({request, fusion, onClose}: {request: UploadRequest; fusion: boolean; onClose: () => void}) {
    const intl = useIntl();
    const label = intl.formatMessage(request.kind === 'sticker' ? messages.addSticker : messages.addGif);
    return (
        <button
            type='button'
            className={fusion ? 'am-icon-btn' : 'amgifs-add-btn'}
            {...iconTip(label)}
            onClick={() => {
                onClose();
                openUpload(request);
            }}
        >
            <PlusIcon className={fusion ? 'am-ic' : 'amgifs-ic'}/>
        </button>
    );
}

// UploadDialogRoot shows the upload dialog while it's open (see openUpload); registered as a root
// component, so that it outlives the picker it is opened from.
export default function UploadDialogRoot() {
    const request = useUpload();
    if (!request) {
        return null;
    }
    const fusion = isFusionUI();
    const dialog = (
        <UploadDialog
            key={`${request.kind}:${request.channelId}:${request.rootId}`}
            request={request}
            fusion={fusion}
        />
    );
    return ReactDOM.createPortal(fusion ? dialog : <div className='amgifs-layer'>{dialog}</div>, layerContainer(fusion));
}
