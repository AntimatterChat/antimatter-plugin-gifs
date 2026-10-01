// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React from 'react';

type Props = {className?: string};

// Line icons drawn with the current color, sized by their class (am-ic under Fusion).
function Svg({className, children}: Props & {children: React.ReactNode}) {
    return (
        <svg
            className={className}
            viewBox='0 0 24 24'
            width='16'
            height='16'
            fill='none'
            stroke='currentColor'
            strokeWidth='1.8'
            strokeLinecap='round'
            strokeLinejoin='round'
            aria-hidden='true'
        >
            {children}
        </svg>
    );
}

export function StarIcon(props: Props) {
    return (
        <Svg {...props}>
            <path d='m12 3.5 2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9l-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z'/>
        </Svg>
    );
}

export function SearchIcon(props: Props) {
    return (
        <Svg {...props}>
            <circle
                cx='11'
                cy='11'
                r='7'
            />
            <path d='m20 20-3.5-3.5'/>
        </Svg>
    );
}

export function PlusIcon(props: Props) {
    return (
        <Svg {...props}>
            <path d='M12 5v14M5 12h14'/>
        </Svg>
    );
}

// A sticker: a square with a peeled corner and a smile.
export function StickerIcon(props: Props) {
    return (
        <Svg {...props}>
            <path d='M5 4h14a1 1 0 0 1 1 1v9l-6 6H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z'/>
            <path d='M14 20v-5a1 1 0 0 1 1-1h5M9 10h.01M15 10h.01M9 14.5a4 4 0 0 0 3 1'/>
        </Svg>
    );
}

// GIF in a rounded frame.
export function GifIcon(props: Props) {
    return (
        <Svg {...props}>
            <rect
                x='2.5'
                y='5'
                width='19'
                height='14'
                rx='3'
            />
            <text
                x='12'
                y='15'
                textAnchor='middle'
                fontSize='7.5'
                fontWeight='800'
                fill='currentColor'
                stroke='none'
            >
                {'GIF'}
            </text>
        </Svg>
    );
}
