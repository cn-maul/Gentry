import type { ValidationResult } from '@/api/types'

interface MonitorValidationPanelProps {
  result?: ValidationResult | null
  loading?: boolean
}

export default function MonitorValidationPanel({ result, loading = false }: MonitorValidationPanelProps) {
  if (!loading && !result) return null

  return (
    <div className="validation-panel">
      {loading ? (
        <div className="loading">
          <div className="spinner" />
          <p>正在抓取网页并验证配置...</p>
        </div>
      ) : (
        result && (
          <>
            <div className="section-header row">
              <h2>验证结果</h2>
              <span className={`validation-status ${result.valid ? 'status-ok' : 'status-err'}`}>
                {result.valid ? '验证通过' : '存在问题'}
              </span>
            </div>

            {result.items && result.items.length > 0 && (
              <div className="validation-grid">
                {result.items.slice(0, 5).map((item, idx) => (
                  <div key={idx} className={`validation-item item-${item.status}`}>
                    <div className="item-header">
                      <span className="item-status-icon">
                        {item.status === 'ok' && (
                          <svg viewBox="0 0 24 24" fill="currentColor" width="14" height="14">
                            <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" />
                          </svg>
                        )}
                        {item.status === 'warn' && (
                          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
                            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
                            <line x1="12" y1="9" x2="12" y2="13" />
                            <line x1="12" y1="17" x2="12.01" y2="17" />
                          </svg>
                        )}
                        {item.status === 'err' && (
                          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
                            <circle cx="12" cy="12" r="10" />
                            <line x1="15" y1="9" x2="9" y2="15" />
                            <line x1="9" y1="9" x2="15" y2="15" />
                          </svg>
                        )}
                      </span>
                      <span className="item-label">{item.label}</span>
                    </div>
                    <p className="item-detail">{item.detail}</p>
                    {item.samples && item.samples.length > 0 && (
                      <div className="item-samples">
                        {item.samples.slice(0, 3).map((s, si) => (
                          <div key={si} className="sample-row">
                            <span className="sample-key" title={s.item_key}>
                              {s.item_key}
                            </span>
                            <span className="sample-raw">{s.raw}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}

            {result.errors && result.errors.length > 0 && (
              <div className="validation-errors">
                {result.errors.map((err, idx) => (
                  <div key={idx} className="error-item">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
                      <circle cx="12" cy="12" r="10" />
                      <line x1="15" y1="9" x2="9" y2="15" />
                      <line x1="9" y1="9" x2="15" y2="15" />
                    </svg>
                    <span>{err}</span>
                  </div>
                ))}
              </div>
            )}

            {result.summary && (
              <div className="validation-summary">
                <p>{result.summary}</p>
              </div>
            )}
          </>
        )
      )}
    </div>
  )
}
