/** How long the object URL outlives the click; revoking at once can cancel the download. */
const REVOKE_DELAY_MS = 1000

/** Saves blob as filename through a temporary `<a download>` link. */
export function downloadBlob(filename: string, blob: Blob): void {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), REVOKE_DELAY_MS)
}
