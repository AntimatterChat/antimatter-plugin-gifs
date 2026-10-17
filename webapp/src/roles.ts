// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import type {GlobalState} from '@mattermost/types/store';

// isGuest tells whether the current user is a guest account (the system_guest role). Like custom
// emoji, guests neither add nor delete GIFs and stickers: the server refuses them, and the picker
// hides its add and manage buttons from them.
export function isGuest(state: GlobalState): boolean {
    const users = state?.entities?.users;
    const me = users?.profiles?.[users.currentUserId];
    return (me?.roles || '').split(' ').includes('system_guest');
}
