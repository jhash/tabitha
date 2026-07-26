import { useNavigate } from '@tanstack/react-router'

import { transposeLabel, wrapSemitones } from '~/lib/transpose'

/**
 * Port of internal/web/transcription_render.go's `transposeControlsNode`
 * plus the stepper half of static/js/transpose.js.
 *
 * The offset lives in the URL (`?t=`) exactly as it did before, which is
 * what makes a transposed view refresh-safe, shareable, and carried
 * across the Show <-> Play navigation. The difference is that the Go
 * version had to hand-patch every outgoing link's href to keep the param
 * attached; here the router's `search` inheritance does it, so the
 * `updateTransposeLinks` machinery has no successor.
 */

export interface TransposeControlsProps {
  /** The song's stored key; "" when unknown, which shows a bare offset. */
  songKey: string
  semitones: number
}

export function TransposeControls({ songKey, semitones }: TransposeControlsProps) {
  const navigate = useNavigate()

  const step = (delta: number) => {
    const next = wrapSemitones(semitones + delta)
    void navigate({
      to: '.',
      // Drop the param entirely at 0 so an untransposed URL stays clean.
      search: (prev: Record<string, unknown>) => ({
        ...prev,
        t: next === 0 ? undefined : next,
      }),
      // replace, not push: the stepper fires on every click, and each
      // semitone isn't a place the back button should stop at.
      replace: true,
      // The chords re-render from data already in cache — there's nothing
      // to refetch, and resetting scroll mid-song would be hostile.
      resetScroll: false,
    })
  }

  return (
    <div className="transpose-controls" data-key={songKey}>
      <button
        type="button"
        className="transpose-down"
        aria-label="Transpose down a semitone"
        onClick={() => step(-1)}
      >
        −
      </button>
      <span className="transpose-key">{transposeLabel(songKey, semitones)}</span>
      <button
        type="button"
        className="transpose-up"
        aria-label="Transpose up a semitone"
        onClick={() => step(1)}
      >
        +
      </button>
    </div>
  )
}
