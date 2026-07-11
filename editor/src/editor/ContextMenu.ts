interface MenuItem {
  label: string;
  onSelect: () => void;
}

let closeActiveMenu: (() => void) | null = null;

export function openContextMenu(x: number, y: number, items: MenuItem[]): void {
  closeActiveMenu?.();

  const root = document.createElement("ul");
  root.className = "chord-context-menu";
  root.style.position = "fixed";
  root.style.left = `${x}px`;
  root.style.top = `${y}px`;

  for (const item of items) {
    const el = document.createElement("li");
    el.textContent = item.label;
    el.className = "chord-context-menu-item";
    el.addEventListener("mousedown", (e) => {
      e.preventDefault();
      cleanup();
      item.onSelect();
    });
    root.appendChild(el);
  }

  function cleanup() {
    document.removeEventListener("mousedown", onDocumentMouseDown, true);
    root.remove();
    closeActiveMenu = null;
  }

  function onDocumentMouseDown(e: MouseEvent) {
    if (!root.contains(e.target as Node)) cleanup();
  }

  document.body.appendChild(root);
  setTimeout(() => document.addEventListener("mousedown", onDocumentMouseDown, true), 0);
  closeActiveMenu = cleanup;
}
