import { writable } from 'svelte/store';

const BUBBLES_KEY = 'koalaparty.chatBubbles';
const CAPTIONS_KEY = 'koalaparty.suppressCaptions';

function getBooleanStorage(key: string, defaultValue: boolean): boolean {
  if (typeof localStorage === 'undefined') return defaultValue;
  try {
    const val = localStorage.getItem(key);
    return val !== null ? val === '1' : defaultValue;
  } catch {
    return defaultValue;
  }
}

function setBooleanStorage(key: string, value: boolean) {
  if (typeof localStorage === 'undefined') return;
  try {
    localStorage.setItem(key, value ? '1' : '0');
  } catch {
    /* storage unavailable */
  }
}

export const chatBubblesEnabled = writable<boolean>(getBooleanStorage(BUBBLES_KEY, true));
export const suppressCaptionsEnabled = writable<boolean>(getBooleanStorage(CAPTIONS_KEY, true));

export function setChatBubbles(enabled: boolean) {
  chatBubblesEnabled.set(enabled);
  setBooleanStorage(BUBBLES_KEY, enabled);
}

export function setSuppressCaptions(enabled: boolean) {
  suppressCaptionsEnabled.set(enabled);
  setBooleanStorage(CAPTIONS_KEY, enabled);
}
