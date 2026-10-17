// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React, {useCallback, useId, useState} from 'react';

import type {PickerKit, PickerListItem} from '../src/types/host';

// fakeKit stands in for the Fusion host's picker kit (fusion/popovers/picker_views.tsx) in the
// plugin's tests: the same props and roles, the least markup. The host tests the kit itself.
export const fakeKit: PickerKit = {
    version: 1,
    usePickerView: <V extends string>() => {
        const [view, setView] = useState<V | 'grid'>('grid');
        const open = useCallback((v: V) => setView(v), []);
        const back = useCallback(() => setView('grid'), []);
        return [view, open, back];
    },
    useOnEscape: () => undefined,
    PickerViewButtons: ({onAdd, onManage, addLabel, manageLabel}) => (
        <>
            {onAdd && (
                <button
                    type='button'
                    aria-label={addLabel}
                    onClick={onAdd}
                />
            )}
            {onManage && (
                <button
                    type='button'
                    aria-label={manageLabel}
                    onClick={onManage}
                />
            )}
        </>
    ),
    PickerViewFrame: ({title, onBack, actions, children, onSubmit}) => {
        const body = (
            <>
                <button
                    type='button'
                    aria-label='Back'
                    onClick={onBack}
                />
                <h3>{title}</h3>
                {children}
                {actions}
            </>
        );
        return onSubmit ? (
            <form
                aria-label={title}
                onSubmit={(e) => {
                    e.preventDefault();
                    onSubmit();
                }}
            >
                {body}
            </form>
        ) : <section aria-label={title}>{body}</section>;
    },
    PickerNotice: ({kind = 'error', children}) => (children ? <p role={kind === 'error' ? 'alert' : 'status'}>{children}</p> : null),
    PickerField: ({label, hint, error, children}) => {
        const id = useId();
        return (
            <div>
                <label htmlFor={id}>{label}</label>
                {children({id, describedBy: id + '-m', invalid: Boolean(error)})}
                <p id={id + '-m'}>{error || hint}</p>
            </div>
        );
    },
    PickerSearch: ({value, onChange, placeholder}) => (
        <input
            type='search'
            aria-label={placeholder}
            value={value}
            onChange={(e) => onChange(e.target.value)}
        />
    ),
    PickerDropZone: ({accept, onFile, label, hint, error, preview}) => (
        <div data-testid='drop'>
            <input
                type='file'
                aria-label={label}
                accept={accept}
                onChange={(e) => {
                    const f = e.target.files?.[0];
                    if (f) {
                        onFile(f);
                    }
                }}
            />
            {preview}
            <p role={error ? 'alert' : undefined}>{error || hint}</p>
        </div>
    ),
    PickerManageList: ({items, onDelete, confirmText, deleteLabel, deletedText, empty, loading, hasMore, onLoadMore}) => {
        const [confirming, setConfirming] = useState('');
        const [message, setMessage] = useState('');
        const remove = async (item: PickerListItem) => {
            const why = await onDelete(item);
            setConfirming('');
            setMessage(why || deletedText(item));
        };
        return (
            <>
                {message && <p role='status'>{message}</p>}
                <ul>
                    {items.map((item) => (
                        <li key={item.id}>
                            <span>{item.name}</span>
                            {item.by && <span>{item.by}</span>}
                            {item.canDelete && confirming !== item.id && (
                                <button
                                    type='button'
                                    aria-label={deleteLabel(item)}
                                    onClick={() => setConfirming(item.id)}
                                />
                            )}
                            {confirming === item.id && (
                                <span>
                                    {confirmText(item)}
                                    <button
                                        type='button'
                                        onClick={() => setConfirming('')}
                                    >
                                        {'Keep'}
                                    </button>
                                    <button
                                        type='button'
                                        onClick={() => remove(item)}
                                    >
                                        {'Delete'}
                                    </button>
                                </span>
                            )}
                        </li>
                    ))}
                </ul>
                {!items.length && !loading && <p>{empty}</p>}
                {hasMore && (
                    <button
                        type='button'
                        onClick={onLoadMore}
                    >
                        {'Show more'}
                    </button>
                )}
            </>
        );
    },
};
