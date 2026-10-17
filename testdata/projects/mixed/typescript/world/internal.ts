export interface Reader { read(): string }
export function open(): Reader { return {read: () => 'world'} }
export function secret(): string { return 'secret' }
