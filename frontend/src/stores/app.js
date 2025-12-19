import { writable } from 'svelte/store';

export const projects = writable([]);
export const currentProject = writable(null);
export const code = writable(`// Query your models
$users = User::all();

// Or run any Laravel code
dump($users->count());
`);
export const output = writable('');
export const isRunning = writable(false);
export const layout = writable('horizontal'); // 'horizontal' or 'vertical'

// Snippets for quick access
export const snippets = writable([]);
