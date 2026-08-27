import { Link } from 'react-router'
import type { ScanContainer } from '@/api/types'

interface RuleMatchPanelProps {
  url: string
  matching: boolean
  matched: boolean
  result: { containers?: ScanContainer[] } | null
  matchError: string | null
  appliedIndex: number | null
  onApply: (container: ScanContainer, index: number) => void
  onDismiss: () => void
}

/**
 * 规则匹配结果面板：显示「匹配规则」按钮的命中结果。
 * 命中时展示规则名、适用地址与预抓取内容；未命中时引导去规则库添加规则。
 */
export default function RuleMatchPanel({
  url,
  matching,
  matched,
  result,
  matchError,
  appliedIndex,
  onApply,
  onDismiss,
}: RuleMatchPanelProps) {
  const containers = result?.containers || []
  const noMatch = matched && containers.length === 0

  function displayName(container: ScanContainer) {
    return container.rule_name || container.strategy || '匹配到的内容区域'
  }

  function displayAddress(container: ScanContainer) {
    return container.rule_address || url
  }

  return (
    <div className="rule-match-panel">
      {matching && (
        <div className="loading">
          <div className="spinner" />
          <p>正在按规则识别网页内容...</p>
        </div>
      )}

      {!matching && matched && containers.length > 0 && (
        <div className="scan-results">
          <p className="results-label">命中 {containers.length} 条扫描规则，请确认符合预期的内容区域：</p>
          {containers.map((container, ci) => {
            const applied = appliedIndex === ci
            return (
              <div key={ci} className={`matched-rule${applied ? ' applied' : ''}`}>
                <div className="mr-row">
                  <span className="mr-label">规则名</span>
                  <span className="mr-value mr-strong">{displayName(container)}</span>
                  {container.item_count > 0 && <span className="candidate-count">预抓取 {container.item_count} 条</span>}
                </div>
                <div className="mr-row">
                  <span className="mr-label">适用地址</span>
                  <span className="mr-value mr-address">{displayAddress(container)}</span>
                </div>
                {container.item_count > 0 && (
                  <div className="candidate-samples">
                    {(container.sample_items || []).slice(0, 5).map((item, ii) => (
                      <div key={ii} className="sample-item">
                        <span className="sample-title">{item.title || '未命名条目'}</span>
                        {item.date ? <span className="sample-meta">{item.date}</span> : null}
                      </div>
                    ))}
                    {container.item_count > 5 && <div className="sample-more">...还有 {container.item_count - 5} 条</div>}
                  </div>
                )}
                {applied ? (
                  <div className="selection-confirmed">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
                      <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
                      <polyline points="22 4 12 14.01 9 11.01" />
                    </svg>
                    <span>已套用此规则，系统将监控新出现的内容</span>
                  </div>
                ) : (
                  <div className="mr-actions">
                    <button
                      className="btn btn-sm btn-primary"
                      type="button"
                      onClick={() => onApply(container, ci)}
                    >
                      使用此规则
                    </button>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {!matching && noMatch && (
        <div className="empty-scan">
          <p>该网址没有命中的扫描规则。</p>
          <div className="empty-actions">
            <Link to="/rules/add" className="btn btn-primary btn-sm">
              去添加规则
            </Link>
            <button className="btn btn-ghost btn-sm" type="button" onClick={onDismiss}>
              知道了
            </button>
          </div>
        </div>
      )}

      {matchError && <div className="form-error">{matchError}</div>}
    </div>
  )
}