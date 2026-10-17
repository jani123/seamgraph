import {open as makeWorld} from '@world';
import type {Reader} from '@world';
export function read(): string { const reader: Reader = makeWorld(); return reader.read(); }
