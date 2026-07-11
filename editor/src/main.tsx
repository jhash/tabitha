import { createRoot } from 'react-dom/client'
import './index.css'
import { SongEditor } from './editor/SongEditor'

const mount = document.getElementById('tabitha-editor-root')
if (mount) {
  const songID = mount.dataset.songId
  if (songID) {
    createRoot(mount).render(<SongEditor songID={songID} />)
  }
}
