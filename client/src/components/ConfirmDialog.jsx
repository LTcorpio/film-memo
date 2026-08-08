import Icon from './Icon.jsx';

/**
 * 自定义二次确认弹窗（替代原生 confirm/alert）
 * danger=true 时使用红色警示样式
 */
export default function ConfirmDialog({
  open, title, message, confirmText = '确定', cancelText = '取消',
  danger = false, busy = false, error, onConfirm, onCancel,
}) {
  if (!open) return null;
  return (
    <div className="modal-overlay" onClick={busy ? undefined : onCancel}>
      <div className="modal confirm-dialog" onClick={(e) => e.stopPropagation()}>
        <div className={`confirm-icon${danger ? ' danger' : ''}`}>
          <Icon name={danger ? 'alert' : 'info'} size={26} />
        </div>
        <h3 className="confirm-title">{title}</h3>
        {message && <p className="confirm-message">{message}</p>}
        {error && <div className="confirm-error"><Icon name="alert" size={13} /> {error}</div>}
        <div className="confirm-actions">
          <button className="btn-secondary" disabled={busy} onClick={onCancel}>
            {cancelText}
          </button>
          <button
            className={danger ? 'btn-danger solid' : 'btn-primary'}
            disabled={busy}
            onClick={onConfirm}
          >
            {busy ? '处理中…' : confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}
