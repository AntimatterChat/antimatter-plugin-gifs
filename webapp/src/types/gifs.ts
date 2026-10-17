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

    // Whether the user can add GIFs and stickers (and delete their own).
    can_upload?: boolean;

    // Whether the user can delete anyone's uploads (system admins).
    can_delete_others?: boolean;

    // The largest file an upload takes.
    upload_max_bytes?: number;
};

// A GIF or sticker someone added, as GET /api/v1/uploads lists it.
export type UploadEntry = {
    id: string;
    kind: Kind;
    title: string;
    tags?: string;
    pack?: string;
    user_id: string;

    // Milliseconds since the epoch.
    created_at: number;
    item: Item | null;
    username?: string;
    can_delete: boolean;
};

export type UploadList = {
    uploads: UploadEntry[];
    has_more: boolean;
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
