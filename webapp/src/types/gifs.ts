// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

// The shapes of the plugin's REST API (see server/gifservice.go).

export type Kind = 'gif' | 'sticker';

export type Media = {
    url: string;
    width?: number;
    height?: number;
    video?: boolean;
};

export type Item = {
    id: string;
    kind: Kind;
    title?: string;
    description?: string;
    preview: Media;
    post: Media;
    image?: Media;
    still?: Media;
    attribution?: string;
    source_url?: string;
};

export type Page = {
    items: Item[];
    next?: string;
};

export type Category = {
    name: string;
    searchterm: string;
    image?: string;
};

export type ClientConfig = {
    configured: boolean;
    stickers: boolean;

    // Whether the user can add GIFs and stickers.
    can_upload?: boolean;
};

// A GIF or sticker to add to the GIF service.
export type Upload = {
    kind: Kind;
    file: File;
    title: string;

    // Comma-separated.
    tags: string;

    // The sticker pack of a sticker.
    pack?: string;
};
