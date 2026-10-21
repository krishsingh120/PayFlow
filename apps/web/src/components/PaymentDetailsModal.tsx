import React from 'react';
import { X, Copy, Check, RotateCcw, AlertCircle, CheckCircle2 } from 'lucide-react';
import { formatINR, listRefunds, createRefund } from '../lib/api';
import type { Payment, Refund } from '../lib/api';

interface PaymentDetailsModalProps {
  payment: Payment | null;
  onClose: () => void;
  onPaymentUpdated?: () => void;
}

export const PaymentDetailsModal: React.FC<PaymentDetailsModalProps> = ({ payment, onClose, onPaymentUpdated }) => {
  const [copied, setCopied] = React.useState(false);
  const [refunds, setRefunds] = React.useState<Refund[]>([]);
  const [loadingRefunds, setLoadingRefunds] = React.useState(false);
  const [refundAmountRupees, setRefundAmountRupees] = React.useState('');
  const [refundReason, setRefundReason] = React.useState('');
  const [refunding, setRefunding] = React.useState(false);
  const [refundError, setRefundError] = React.useState<string | null>(null);
  const [refundSuccess, setRefundSuccess] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (payment) {
      setRefundError(null);
      setRefundSuccess(null);
      setRefundAmountRupees('');
      setRefundReason('');
      if (payment.status === 'SUCCEEDED') {
        setLoadingRefunds(true);
        listRefunds(payment.id)
          .then((res) => {
            if (res.data) setRefunds(res.data);
          })
          .finally(() => setLoadingRefunds(false));
      } else {
        setRefunds([]);
      }
    }
  }, [payment]);

  if (!payment) return null;

  const totalRefunded = refunds
    .filter((r) => r.status !== 'FAILED')
    .reduce((sum, r) => sum + r.amount, 0);

  const remainingBalance = Math.max(0, payment.amount - totalRefunded);

  const handleRefundSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setRefundError(null);
    setRefundSuccess(null);

    const rupees = parseFloat(refundAmountRupees);
    if (isNaN(rupees) || rupees <= 0) {
      setRefundError('Please enter a valid refund amount greater than 0');
      return;
    }

    const paise = Math.round(rupees * 100);
    if (paise > remainingBalance) {
      setRefundError(`Refund exceeds available balance (${formatINR(remainingBalance)})`);
      return;
    }

    setRefunding(true);
    const idempotencyKey = `ref_ui_${payment.id}_${Date.now()}`;
    const res = await createRefund(
      payment.id,
      { amount: paise, reason: refundReason || 'Dashboard user refund' },
      idempotencyKey
    );
    setRefunding(false);

    if (res.error) {
      setRefundError(`${res.error}: ${res.message}`);
    } else if (res.data) {
      setRefundSuccess(`Refund of ${formatINR(res.data.amount)} processed successfully!`);
      setRefundAmountRupees('');
      setRefundReason('');
      setRefunds((prev) => [res.data!, ...prev]);
      if (onPaymentUpdated) onPaymentUpdated();
    }
  };

  const handleCopyJSON = () => {
    navigator.clipboard.writeText(JSON.stringify(payment, null, 2));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" style={{ maxWidth: 640 }} onClick={(e) => e.stopPropagation()}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
              <h3 style={{ fontSize: 18, fontWeight: 600, color: '#F8FAFC' }}>Transaction Inspector</h3>
              <span className={`status-badge ${payment.status}`}>{payment.status}</span>
            </div>
            <p style={{ fontSize: 13, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>
              {payment.id}
            </p>
          </div>
          <button
            onClick={onClose}
            style={{ background: 'none', border: 'none', color: 'var(--text-dim)', cursor: 'pointer' }}
          >
            <X size={20} />
          </button>
        </div>

        {/* Details Grid */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1fr 1fr',
            gap: 12,
            background: 'rgba(15, 23, 42, 0.6)',
            padding: 16,
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-subtle)',
            marginBottom: 20,
            fontSize: 13,
          }}
        >
          <div>
            <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Amount</span>
            <strong style={{ color: 'var(--emerald-400)', fontSize: 16 }}>{formatINR(payment.amount)}</strong>{' '}
            <span style={{ fontSize: 11, color: 'var(--text-dim)' }}>({payment.amount} paise)</span>
          </div>

          <div>
            <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Currency</span>
            <strong style={{ color: '#F8FAFC' }}>{payment.currency}</strong>
          </div>

          <div style={{ gridColumn: 'span 2' }}>
            <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Idempotency Key</span>
            <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-muted)', wordBreak: 'break-all' }}>
              {payment.idempotency_key}
            </span>
          </div>

          {payment.description && (
            <div style={{ gridColumn: 'span 2' }}>
              <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Description</span>
              <span style={{ color: '#F8FAFC' }}>{payment.description}</span>
            </div>
          )}

          <div>
            <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Created At</span>
            <span style={{ color: 'var(--text-muted)' }}>{new Date(payment.created_at).toLocaleString()}</span>
          </div>

          <div>
            <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Updated At</span>
            <span style={{ color: 'var(--text-muted)' }}>{new Date(payment.updated_at).toLocaleString()}</span>
          </div>

          {payment.provider_ref && (
            <div>
              <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Provider Reference</span>
              <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--emerald-400)' }}>{payment.provider_ref}</span>
            </div>
          )}

          {payment.failure_reason && (
            <div>
              <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Failure Reason</span>
              <span style={{ color: '#F43F5E' }}>{payment.failure_reason}</span>
            </div>
          )}
        </div>

        {/* Refunds Management Section (Only for SUCCEEDED payments) */}
        {payment.status === 'SUCCEEDED' && (
          <div
            style={{
              marginBottom: 20,
              padding: 16,
              background: 'rgba(15, 23, 42, 0.4)',
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--border-subtle)',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <RotateCcw size={16} color="var(--emerald-400)" />
                <h4 style={{ fontSize: 14, fontWeight: 600, color: '#F8FAFC' }}>Refund Management</h4>
              </div>
              <span style={{ fontSize: 12, color: 'var(--text-dim)' }}>
                {loadingRefunds ? (
                  <span>Loading history...</span>
                ) : (
                  <>Available: <strong style={{ color: remainingBalance > 0 ? 'var(--emerald-400)' : 'var(--text-dim)' }}>{formatINR(remainingBalance)}</strong></>
                )}
              </span>
            </div>

            {/* Refund form */}
            {remainingBalance > 0 ? (
              <form onSubmit={handleRefundSubmit} style={{ display: 'flex', gap: 8, marginBottom: 12, alignItems: 'flex-start' }}>
                <div style={{ flex: 1 }}>
                  <input
                    type="number"
                    step="0.01"
                    min="0.01"
                    max={remainingBalance / 100}
                    placeholder="Amount in ₹"
                    value={refundAmountRupees}
                    onChange={(e) => setRefundAmountRupees(e.target.value)}
                    style={{
                      width: '100%',
                      padding: '8px 12px',
                      borderRadius: 'var(--radius-md)',
                      background: 'var(--bg-subtle)',
                      border: '1px solid var(--border-subtle)',
                      color: '#F8FAFC',
                      fontSize: 13,
                    }}
                  />
                </div>
                <div style={{ flex: 1.5 }}>
                  <input
                    type="text"
                    placeholder="Reason (optional)"
                    value={refundReason}
                    onChange={(e) => setRefundReason(e.target.value)}
                    style={{
                      width: '100%',
                      padding: '8px 12px',
                      borderRadius: 'var(--radius-md)',
                      background: 'var(--bg-subtle)',
                      border: '1px solid var(--border-subtle)',
                      color: '#F8FAFC',
                      fontSize: 13,
                    }}
                  />
                </div>
                <button
                  type="submit"
                  disabled={refunding}
                  className="btn btn-primary"
                  style={{ whiteSpace: 'nowrap', padding: '8px 14px', fontSize: 13 }}
                >
                  {refunding ? 'Processing...' : 'Issue Refund'}
                </button>
              </form>
            ) : (
              <p style={{ fontSize: 12, color: 'var(--text-dim)', marginBottom: 12 }}>
                This transaction has been fully refunded.
              </p>
            )}

            {refundError && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: '#F43F5E', fontSize: 12, marginBottom: 10 }}>
                <AlertCircle size={14} />
                <span>{refundError}</span>
              </div>
            )}

            {refundSuccess && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: 'var(--emerald-400)', fontSize: 12, marginBottom: 10 }}>
                <CheckCircle2 size={14} />
                <span>{refundSuccess}</span>
              </div>
            )}

            {/* List of past refunds */}
            {refunds.length > 0 && (
              <div style={{ borderTop: '1px solid var(--border-subtle)', paddingTop: 10 }}>
                <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--text-dim)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                  Refund History ({refunds.length})
                </span>
                <div style={{ marginTop: 6, display: 'flex', flexDirection: 'column', gap: 6 }}>
                  {refunds.map((r) => (
                    <div
                      key={r.id}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        padding: '6px 10px',
                        background: 'rgba(2, 6, 23, 0.5)',
                        borderRadius: 4,
                        fontSize: 12,
                      }}
                    >
                      <div>
                        <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-muted)' }}>{r.id}</span>
                        {r.reason && <span style={{ marginLeft: 8, color: 'var(--text-dim)' }}>— {r.reason}</span>}
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <span style={{ color: 'var(--emerald-400)', fontWeight: 600 }}>{formatINR(r.amount)}</span>
                        <span className={`status-badge ${r.status}`} style={{ fontSize: 10, padding: '2px 6px' }}>{r.status}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Raw JSON View */}
        <div>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
            <span style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-dim)' }}>Raw JSON Response</span>
            <button
              onClick={handleCopyJSON}
              style={{
                background: 'none',
                border: 'none',
                color: copied ? 'var(--emerald-400)' : 'var(--text-dim)',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: 4,
                fontSize: 12,
              }}
            >
              {copied ? <Check size={12} /> : <Copy size={12} />}
              {copied ? 'Copied' : 'Copy JSON'}
            </button>
          </div>
          <pre
            style={{
              background: '#0B0F19',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-md)',
              padding: 12,
              fontFamily: 'var(--font-mono)',
              fontSize: 11,
              color: 'var(--emerald-400)',
              overflowX: 'auto',
              maxHeight: 200,
            }}
          >
            {JSON.stringify(payment, null, 2)}
          </pre>
        </div>

        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn btn-secondary" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
