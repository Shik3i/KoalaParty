// Shared DOM actions for room menus and dialogs.

/**
 * Positions a `<details>` menu as a fixed popover under its trigger so scrolling,
 * clipped panels never hide it, and closes it on outside clicks.
 */
export function anchoredMenu(details: HTMLDetailsElement) {
  const menu = details.querySelector<HTMLElement>('.menu');
  const reposition = () => {
    if (!menu || !details.open) return;
    const rect = details.getBoundingClientRect();
    menu.style.top = `${rect.bottom + 4}px`;
    menu.style.right = `${window.innerWidth - rect.right}px`;
  };
  const closeOutside = (event: MouseEvent) => {
    if (details.open && !details.contains(event.target as Node)) details.open = false;
  };
  details.addEventListener('toggle', reposition);
  window.addEventListener('scroll', reposition, true);
  window.addEventListener('resize', reposition);
  document.addEventListener('click', closeOutside);
  return {
    destroy() {
      details.removeEventListener('toggle', reposition);
      window.removeEventListener('scroll', reposition, true);
      window.removeEventListener('resize', reposition);
      document.removeEventListener('click', closeOutside);
    },
  };
}

/** Closes the `<details>` menu that contains the event target. */
export function closeMenu(event: Event) {
  (event.currentTarget as HTMLElement).closest('details')?.removeAttribute('open');
}

/** Keeps keyboard focus inside a dialog and restores it afterwards. */
export function focusTrap(node: HTMLElement, onEscape: () => void) {
  const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const controls = () =>
    Array.from(node.querySelectorAll<HTMLElement>('button:not([disabled]), a[href], input, select, textarea'));
  let escape = onEscape;
  const keydown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      escape();
      return;
    }
    if (event.key !== 'Tab') return;
    const items = controls();
    if (!items.length) return;
    const first = items[0];
    const last = items[items.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };
  node.addEventListener('keydown', keydown);
  requestAnimationFrame(() => controls().at(-1)?.focus());
  return {
    update(next: () => void) {
      escape = next;
    },
    destroy() {
      node.removeEventListener('keydown', keydown);
      previous?.focus();
    },
  };
}
