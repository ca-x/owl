import { useMemo, useState } from 'react'

import type { DictionarySummary } from '../types'

interface DictionaryManagerPageProps {
  dictionaries: DictionarySummary[]
  loading: boolean
  error: string
  onRefresh: () => Promise<void>
  onUpload: (mdxFile: File, mddFiles: File[]) => Promise<void>
  onToggle: (dictionary: DictionarySummary) => Promise<void>
  onDelete: (dictionary: DictionarySummary) => Promise<void>
}

export function DictionaryManagerPage({
  dictionaries,
  loading,
  error,
  onRefresh,
  onUpload,
  onToggle,
  onDelete,
}: DictionaryManagerPageProps) {
  const [mdxFile, setMdxFile] = useState<File | null>(null)
  const [mddFiles, setMddFiles] = useState<File[]>([])
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState('')

  const enabledCount = useMemo(() => dictionaries.filter((item) => item.enabled).length, [dictionaries])

  async function handleUpload(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!mdxFile) {
      setUploadError('Please choose an MDX file first.')
      return
    }
    setUploading(true)
    setUploadError('')
    try {
      await onUpload(mdxFile, mddFiles)
      setMdxFile(null)
      setMddFiles([])
      await onRefresh()
    } catch (uploadErr) {
      setUploadError(uploadErr instanceof Error ? uploadErr.message : 'Upload failed')
    } finally {
      setUploading(false)
    }
  }

  return (
    <section className="page-section">
      <div className="page-header">
        <div>
          <div className="eyebrow">Dictionary Manager</div>
          <h2>Upload and control your personal dictionaries</h2>
          <p className="muted">
            {dictionaries.length} uploaded · {enabledCount} enabled
          </p>
        </div>
        <button className="secondary-button" type="button" onClick={() => void onRefresh()} disabled={loading}>
          Refresh
        </button>
      </div>

      <form className="card upload-card" onSubmit={handleUpload}>
        <div className="upload-grid">
          <label className="dropzone">
            <span className="dropzone-title">MDX file</span>
            <span className="muted">Required. This contains the dictionary entries.</span>
            <input
              type="file"
              accept=".mdx"
              onChange={(event) => setMdxFile(event.target.files?.[0] ?? null)}
              required
            />
            <strong>{mdxFile ? mdxFile.name : 'Choose .mdx file'}</strong>
          </label>

          <label className="dropzone">
            <span className="dropzone-title">MDD resources</span>
            <span className="muted">Optional. Add paired images, audio, CSS, fonts.</span>
            <input
              type="file"
              accept=".mdd"
              multiple
              onChange={(event) => setMddFiles(Array.from(event.target.files ?? []))}
            />
            <strong>{mddFiles.length > 0 ? `${mddFiles.length} file(s) selected` : 'Choose .mdd file(s)'}</strong>
          </label>
        </div>

        {uploadError ? <div className="error-banner">{uploadError}</div> : null}
        {error ? <div className="error-banner">{error}</div> : null}

        <div className="actions-row">
          <button className="primary-button" type="submit" disabled={uploading}>
            {uploading ? 'Uploading…' : 'Upload dictionary'}
          </button>
        </div>
      </form>

      <div className="dictionary-grid">
        {dictionaries.map((item) => (
          <article key={item.id} className="card dictionary-card">
            <div className="dictionary-card-head">
              <div>
                <h3>{item.title || item.name}</h3>
                <p className="muted">{item.description || 'No description available.'}</p>
              </div>
              <span className={item.enabled ? 'status-pill active' : 'status-pill muted-pill'}>
                {item.enabled ? 'Enabled' : 'Disabled'}
              </span>
            </div>

            <dl className="meta-grid">
              <div>
                <dt>Entries</dt>
                <dd>{item.entry_count}</dd>
              </div>
              <div>
                <dt>MDD files</dt>
                <dd>{item.mdd_paths.length}</dd>
              </div>
              <div>
                <dt>Uploaded</dt>
                <dd>{new Date(item.created_at).toLocaleString()}</dd>
              </div>
              <div>
                <dt>Owner</dt>
                <dd>{item.owner_name || 'You'}</dd>
              </div>
            </dl>

            <div className="actions-row wrap">
              <button className="secondary-button" type="button" onClick={() => void onToggle(item)}>
                {item.enabled ? 'Disable' : 'Enable'}
              </button>
              <button className="danger-button" type="button" onClick={() => void onDelete(item)}>
                Delete
              </button>
            </div>
          </article>
        ))}

        {dictionaries.length === 0 ? (
          <div className="card empty-state">
            <h3>No dictionaries yet</h3>
            <p className="muted">Upload your first MDX and optional MDD pair to start searching.</p>
          </div>
        ) : null}
      </div>
    </section>
  )
}
