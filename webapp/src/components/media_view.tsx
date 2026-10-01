// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';

import type {Media} from '../types/gifs';
import {mediaSrc} from '../urls';

function prefersReducedMotion() {
    return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

// usePrefersReducedMotion follows the user's reduced motion setting.
function usePrefersReducedMotion() {
    const [reduced, setReduced] = useState(prefersReducedMotion);
    useEffect(() => {
        if (typeof window.matchMedia !== 'function') {
            return () => null;
        }
        const query = window.matchMedia('(prefers-reduced-motion: reduce)');
        const onChange = () => setReduced(query.matches);
        query.addEventListener?.('change', onChange);
        return () => query.removeEventListener?.('change', onChange);
    }, []);
    return reduced;
}

// LoopingVideo plays a muted, looped video while it's on screen.
function LoopingVideo({src, poster, label, className}: {src: string; poster: string | null; label: string; className?: string}) {
    const ref = useRef<HTMLVideoElement>(null);
    useEffect(() => {
        const video = ref.current;
        if (!video || typeof IntersectionObserver === 'undefined') {
            return () => null;
        }
        const observer = new IntersectionObserver(([entry]) => {
            if (entry.isIntersecting) {
                video.play?.()?.catch?.(() => null);
            } else {
                video.pause?.();
            }
        });
        observer.observe(video);
        return () => observer.disconnect();
    }, []);
    return (
        <video
            ref={ref}
            className={className}
            src={src}
            {...(poster ? {poster} : {})}
            autoPlay={true}
            loop={true}
            muted={true}
            playsInline={true}
            preload='auto'
            aria-label={label}
        />
    );
}

type Props = {
    media: Media;
    still?: Media;
    label: string;
    className?: string;
};

// MediaView shows an animated GIF, WebP or video, or its still frame for people who prefer
// reduced motion.
export default function MediaView({media, still, label, className}: Props) {
    const reduced = usePrefersReducedMotion();
    const src = mediaSrc(media.url);
    const stillSrc = mediaSrc(still?.url);
    if (!src) {
        return null;
    }
    if (reduced && stillSrc) {
        return (
            <img
                className={className}
                src={stillSrc}
                alt={label}
                loading='lazy'
            />
        );
    }
    if (media.video) {
        return (
            <LoopingVideo
                src={src}
                poster={stillSrc}
                label={label}
                className={className}
            />
        );
    }
    return (
        <img
            className={className}
            src={src}
            alt={label}
            loading='lazy'
            decoding='async'
        />
    );
}
