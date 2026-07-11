// A generated (not exhaustive) list of common chord names for the picker's
// search dropdown — users can still type any chord the list doesn't cover,
// since the picker's search input's raw text is itself a valid selection.
const ROOTS = ["C", "C#", "Db", "D", "D#", "Eb", "E", "F", "F#", "Gb", "G", "G#", "Ab", "A", "A#", "Bb", "B"];
const QUALITIES = ["", "m", "7", "m7", "maj7", "dim", "aug", "sus2", "sus4", "6", "m6", "9", "add9", "m9"];

export const CHORD_LIST: string[] = ROOTS.flatMap((root) => QUALITIES.map((q) => `${root}${q}`));

export function filterChords(query: string): string[] {
  const q = query.trim().toLowerCase();
  if (!q) return CHORD_LIST;
  return CHORD_LIST.filter((c) => c.toLowerCase().includes(q));
}
