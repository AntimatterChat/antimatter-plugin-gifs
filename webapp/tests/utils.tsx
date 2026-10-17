// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {render} from '@testing-library/react';
import React from 'react';
import {IntlProvider} from 'react-intl';
import {Provider} from 'react-redux';
import {createStore} from 'redux';

import type {Item} from '../src/types/gifs';

const state = {
    entities: {
        general: {config: {DefaultClientLocale: 'en'}},
        users: {currentUserId: 'user1', profiles: {user1: {id: 'user1', locale: 'fr'}}},
    },
};

// renderWithProviders renders like the host: with its Intl and Redux providers, as a user of the given roles.
export function renderWithProviders(ui: React.ReactElement, roles = 'system_user') {
    const store = createStore(() => ({...state, entities: {...state.entities, users: {...state.entities.users, profiles: {user1: {...state.entities.users.profiles.user1, roles}}}}}));
    return render(
        <Provider store={store}>
            <IntlProvider locale='en'>{ui}</IntlProvider>
        </Provider>,
    );
}

const media = (path: string, width = 220, height = 124) => ({url: `/plugins/com.antimatterchat.gifs/media/${path}`, width, height});

export function makeGif(id: string, title = 'Happy cat'): Item {
    return {
        id,
        kind: 'gif',
        title,
        preview: media(`${id}/tinygif.gif`),
        post: {...media(`${id}/mp4.mp4`, 498, 280), video: true},
        image: media(`${id}/gif.gif`, 498, 280),
        still: media(`${id}/preview.png`, 498, 280),
        attribution: 'Via Tenor',
        source_url: 'https://tenor.com/view/x',
    };
}

export function makeSticker(id: string, title = 'It works!'): Item {
    return {id, kind: 'sticker', title, preview: media(`${id}/tinywebp_transparent.webp`, 220, 220), post: media(`${id}/tinywebp_transparent.webp`, 220, 220)};
}
