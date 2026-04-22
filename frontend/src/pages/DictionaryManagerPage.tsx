import { useMemo, useState } from 'react'

import { useI18n } from '../i18n'
import type { DictionarySummary, MaintenanceReport } from '../types'

interface DictionaryManagerPageProps {
  dictionaries: DictionarySummary[]
  loading: boolean
  error: string
  maintenanceReport: MaintenanceReport | null
  onRefresh: () => Promise<void>
  onRefreshLibrary: () => Promise<void>
  onUpload: (mdxFile: File, mddFiles: File[]) => Promise<void>
  onToggle: (dictionary: DictionarySummary) => Promise<void>
  onTogglePublic: (dictionary: DictionarySummary) => Promise<void>
  onRefreshItem: (dictionary: DictionarySummary) => Promise<void>
  onDelete: (dictionary: DictionarySummary) => Promise<void>
}

export function DictionaryManagerPage({
  dictionaries,
  loading,
  error,
  maintenanceReport,
  onRefresh,
  onRefreshLibrary,
  onUpload,
  onToggle,
  onTogglePublic,
  onRefreshItem,
  onDelete,
}: DictionaryManagerPageProps) {
  const { t } = useI18n()
  const [mdxFile, setMdxFile] = useState<File | null>(null)
  const [mddFiles, setMddFiles] = useState<File[]>([])
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState('')

  const enabledCount = useMemo(() => dictionaries.filter((item) => item.enabled).length, [dictionaries])

  async function handleUpload(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!mdxFile) {
      setUploadError(t.mdxFileHint)
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
          <div className="eyebrow">{t.dictionaryManager}</div>
          <h2>{t.managerTitle}</h2>
          <p className="muted">{t.uploadedEnabled(dictionaries.length, enabledCount)}</p>
          <p className="muted">{t.maintenanceTip}</p>
        </div>
        <div className="actions-row wrap">
          <button className="secondary-button" type="button" onClick={() => void onRefresh()} disabled={loading}>
            {t.refresh}
          </button>
          <button className="secondary-button" type="button" onClick={() => void onRefreshLibrary()} disabled={loading}>
            {t.refreshLibrary}
          </button>
        </div>
      </div>

      <form className="card upload-card" onSubmit={handleUpload}>
        <div className="upload-grid">
          <label className="dropzone">
            <span className="dropzone-title">{t.mdxFile}</span>
            <span className="muted">{t.mdxFileHint}</span>
            <input
              type="file"
              accept=".mdx"
              onChange={(event) => setMdxFile(event.target.files?.[0] ?? null)}
              required
            />
            <strong>{mdxFile ? mdxFile.name : t.chooseMdx}</strong>
          </label>

          <label className="dropzone">
            <span className="dropzone-title">{t.mddResources}</span>
            <span className="muted">{t.mddHint}</span>
            <input
              type="file"
              accept=".mdd"
              multiple
              onChange={(event) => setMddFiles(Array.from(event.target.files ?? []))}
            />
            <strong>{mddFiles.length > 0 ? `${mddFiles.length} file(s) selected` : t.chooseMdd}</strong>
          </label>
        </div>

        {uploadError ? <div className="error-banner">{uploadError}</div> : null}
        {error ? <div className="error-banner">{error}</div> : null}

        <div className="actions-row">
          <button className="primary-button" type="submit" disabled={uploading}>
            {uploading ? t.uploading : t.uploadDictionary}
          </button>
        </div>
      </form>

      {maintenanceReport ? (
        <section className="card maintenance-report-card">
          <div className="result-group-header">
            <div>
              <div className="eyebrow">{t.maintenanceReportTitle}</div>
              <h3>{maintenanceReport.summary}</h3>
            </div>
          </div>
          <div className="maintenance-stats">
            <span>{t.discoveredCount}: {maintenanceReport.discovered}</span>
            <span>{t.updatedCount}: {maintenanceReport.updated}</span>
            <span>{t.skippedCount}: {maintenanceReport.skipped}</span>
            <span>{t.failedCount}: {maintenanceReport.failed}</span>
          </div>
          <div className="maintenance-item-list">
            {maintenanceReport.items.map((item, index) => (
              <article
                key={`${item.name}-${item.status}-${index}`}
                className={item.dictionary_id ? 'maintenance-item actionable-maintenance-item' : 'maintenance-item'}
                onClick={() => {
                  if (!item.dictionary_id) return
                  const target = document.getElementById(`dictionary-card-${item.dictionary_id}`)
                  target?.scrollIntoView({ behavior: 'smooth', block: 'center' })
                }}
              >
                <strong>{item.name}</strong>
                <p className="muted">{item.message}</p>
              </article>
            ))}
          </div>
        </section>
      ) : null}

      <div className="dictionary-grid">
        {dictionaries.map((item) => {
          const mddPaths = item.mdd_paths ?? []
          const missingFiles = item.missing_files ?? []
          return (
          <article key={item.id} id={`dictionary-card-${item.id}`} className="card dictionary-card">
            <div className="dictionary-card-head">
              <div>
                <h3>{item.title || item.name}</h3>
                <p className="muted">{item.description || t.noDescription}</p>
              </div>
              <span className={item.enabled ? 'status-pill active' : 'status-pill muted-pill'}>
                {item.enabled ? t.enabled : t.disabled}
              </span>
            </div>

            <div className="dictionary-status-row">
              <span className={item.public ? 'status-pill info-pill' : 'status-pill muted-pill'}>
                {item.public ? t.public : t.private}
              </span>
              <span className={statusClassName(item.file_status)}>
                {statusLabel(item.file_status, t)}
              </span>
            </div>

            {missingFiles.length > 0 ? (
              <div className="warning-banner">
                <strong>{t.missingFiles}</strong>
                <ul className="missing-file-list">
                  {missingFiles.map((path) => (
                    <li key={path}>{path}</li>
                  ))}
                </ul>
              </div>
            ) : null}

            <dl className="meta-grid">
              <div>
                <dt>{t.entries}</dt>
                <dd>{item.entry_count}</dd>
              </div>
              <div>
                <dt>{t.mddFiles}</dt>
                <dd>{mddPaths.length}</dd>
              </div>
              <div>
                <dt>{t.uploadedAt}</dt>
                <dd>{new Date(item.created_at).toLocaleString()}</dd>
              </div>
              <div>
                <dt>{t.owner}</dt>
                <dd>{item.owner_name || t.you}</dd>
              </div>
            </dl>

            <div className="actions-row wrap">
              <button className="secondary-button" type="button" onClick={() => void onToggle(item)}>
                {item.enabled ? t.disable : t.enable}
              </button>
              <button className="secondary-button" type="button" onClick={() => void onTogglePublic(item)}>
                {item.public ? t.makePrivate : t.makePublic}
              </button>
              <button className="secondary-button" type="button" onClick={() => void onRefreshItem(item)}>
                {t.refreshItem}
              </button>
              <button className="danger-button" type="button" onClick={() => void onDelete(item)}>
                {t.delete}
              </button>
            </div>
          </article>
        )})}

        {dictionaries.length === 0 ? (
          <div className="card empty-state">
            <h3>{t.noDictionariesYet}</h3>
            <p className="muted">{t.uploadFirstDictionary}</p>
          </div>
        ) : null}
      </div>
    </section>
  )
}

function statusClassName(status: DictionarySummary['file_status']) {
  switch (status) {
    case 'missing_mdx':
    case 'missing_all':
      return 'status-pill danger-pill'
    case 'missing_mdd':
      return 'status-pill warning-pill'
    default:
      return 'status-pill active'
  }
}

function statusLabel(status: DictionarySummary['file_status'], t: ReturnType<typeof useI18n>['t']) {
  switch (status) {
    case 'missing_mdx':
      return t.fileStatusMissingMdx
    case 'missing_mdd':
      return t.fileStatusMissingMdd
    case 'missing_all':
      return t.fileStatusMissingAll
    default:
      return t.fileStatusOk
  }
}
