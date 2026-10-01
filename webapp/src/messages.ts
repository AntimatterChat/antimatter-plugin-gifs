// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {defineMessages} from 'react-intl';

export const messages = defineMessages({
    dialog: {id: 'gifs.picker.dialog', defaultMessage: 'GIFs and stickers'},
    tabGifs: {id: 'gifs.picker.tab.gifs', defaultMessage: 'GIFs'},
    tabStickers: {id: 'gifs.picker.tab.stickers', defaultMessage: 'Stickers'},
    searchGifs: {id: 'gifs.picker.search.gifs', defaultMessage: 'Search GIFs'},
    searchStickers: {id: 'gifs.picker.search.stickers', defaultMessage: 'Search stickers'},
    categories: {id: 'gifs.picker.categories', defaultMessage: 'GIF categories'},
    suggestions: {id: 'gifs.picker.suggestions', defaultMessage: 'Suggested searches'},
    favorites: {id: 'gifs.picker.favorites', defaultMessage: 'Favorites'},
    recent: {id: 'gifs.picker.recent', defaultMessage: 'Recently used'},
    trending: {id: 'gifs.picker.trending', defaultMessage: 'Trending'},
    sendGif: {id: 'gifs.picker.send_gif', defaultMessage: 'Send GIF: {title}'},
    sendSticker: {id: 'gifs.picker.send_sticker', defaultMessage: 'Send sticker: {title}'},
    addFavorite: {id: 'gifs.favorite.add', defaultMessage: 'Add to favorites'},
    removeFavorite: {id: 'gifs.favorite.remove', defaultMessage: 'Remove from favorites'},
    noGifs: {id: 'gifs.picker.no_gifs', defaultMessage: 'No GIFs match.'},
    noStickers: {id: 'gifs.picker.no_stickers', defaultMessage: 'No stickers match.'},
    noFavorites: {id: 'gifs.picker.no_favorites', defaultMessage: 'Star a GIF to keep it here.'},
    noFavoriteStickers: {id: 'gifs.picker.no_favorite_stickers', defaultMessage: 'Star a sticker to keep it here.'},
    noRecent: {id: 'gifs.picker.no_recent', defaultMessage: 'The GIFs you send show here.'},
    loading: {id: 'gifs.picker.loading', defaultMessage: 'Loading…'},
    loadFailed: {id: 'gifs.picker.load_failed', defaultMessage: 'The GIF service isn’t available right now.'},
    postFailed: {id: 'gifs.picker.post_failed', defaultMessage: 'Couldn’t send it: {error}'},
    notConfigured: {id: 'gifs.picker.not_configured', defaultMessage: 'GIFs aren’t set up yet: ask a system admin to configure the GIFs plugin.'},
    note: {id: 'gifs.picker.note', defaultMessage: 'From the server’s GIF provider'},
    close: {id: 'gifs.picker.close', defaultMessage: 'Close'},
    gif: {id: 'gifs.post.gif', defaultMessage: 'GIF'},
    gifLabel: {id: 'gifs.post.gif_label', defaultMessage: 'GIF: {title}'},
    stickerLabel: {id: 'gifs.post.sticker_label', defaultMessage: 'Sticker: {title}'},
    menuItem: {id: 'gifs.menu.item', defaultMessage: 'GIF or sticker'},
    headerButton: {id: 'gifs.header.button', defaultMessage: 'GIFs and stickers'},
});
