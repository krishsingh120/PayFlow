import React, { useState } from 'react';
import { X, Scale, CheckCircle2, AlertTriangle, RefreshCw, ShieldCheck } from 'lucide-react';
import { runReconciliation } from '../lib/api';
import type { ReconciliationReport } from '../lib/api';

interface ReconciliationModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export const ReconciliationModal: React.FC<ReconciliationModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
}) => {
  const [report, setReport] = useState<ReconciliationReport | null>(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleRunReconciliation = async () => {
    setRunning(true);
    setError(null);
    try {
      const res = await runReconciliation();
      if (res.error) {
        setError(`${res.error}: ${res.message}`);
      } else if (res.data) {
        setReport(res.data);
        onSuccess();
      }
    } catch (err: any) {
      setError(err.message || 'Failed to execute reconciliation');
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" style={{ maxWidth: 680 }} onClick={(e) => e.stopPropagation()}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div
              style={{
                width: 36,
                height: 36,
                borderRadius: 8,
                background: 'rgba(56, 189, 248, 0.15)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: '#38BDF8',
              }}
            >
              <Scale size={20} />
            </div>
            <div>
              <h3 style={{ fontSize: 18, fontWeight: 600, color: '#F8FAFC' }}>
                Automated Reconciliation Engine
              </h3>
              <p style={{ fontSize: 12, color: 'var(--text-dim)' }}>
                Audits double-entry ledger balance invariants & resolves UNKNOWN timeout transactions
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            style={{ background: 'none', border: 'none', color: 'var(--text-dim)', cursor: 'pointer' }}
          >
            <X size={20} />
          </button>
        </div>

        {/* Action Banner */}
        <div
          style={{
            padding: 16,
            borderRadius: 'var(--radius-md)',
            background: 'rgba(15, 23, 42, 0.6)',
            border: '1px solid var(--border-subtle)',
            marginBottom: 20,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <div>
            <div style={{ fontSize: 13, fontWeight: 500, color: '#F8FAFC', marginBottom: 2 }}>
              Run Automated Audit Cycle
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-dim)' }}>
              Queries bank settlement switches and verifies Sum(Debits) == Sum(Credits)
            </div>
          </div>
          <button
            className="btn btn-primary"
            onClick={handleRunReconciliation}
            disabled={running}
            style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '8px 16px' }}
          >
            <RefreshCw size={14} style={{ animation: running ? 'spin 1s linear infinite' : 'none' }} />
            {running ? 'Reconciling...' : 'Run Audit'}
          </button>
        </div>

        {error && (
          <div
            style={{
              padding: 12,
              borderRadius: 'var(--radius-md)',
              background: 'rgba(244, 63, 94, 0.15)',
              border: '1px solid rgba(244, 63, 94, 0.3)',
              color: '#F43F5E',
              fontSize: 12,
              marginBottom: 16,
              display: 'flex',
              alignItems: 'center',
              gap: 8,
            }}
          >
            <AlertTriangle size={16} />
            <span>{error}</span>
          </div>
        )}

        {/* Report Output */}
        {report && (
          <div>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(3, 1fr)',
                gap: 12,
                marginBottom: 16,
              }}
            >
              <div
                style={{
                  padding: 12,
                  background: 'rgba(2, 6, 23, 0.5)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border-subtle)',
                }}
              >
                <span style={{ fontSize: 11, color: 'var(--text-dim)', display: 'block' }}>Audited Records</span>
                <strong style={{ fontSize: 18, color: '#F8FAFC' }}>{report.payments_audited}</strong>
              </div>

              <div
                style={{
                  padding: 12,
                  background: 'rgba(2, 6, 23, 0.5)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border-subtle)',
                }}
              >
                <span style={{ fontSize: 11, color: 'var(--text-dim)', display: 'block' }}>Resolved Unknowns</span>
                <strong style={{ fontSize: 18, color: '#38BDF8' }}>{report.unknowns_resolved}</strong>
              </div>

              <div
                style={{
                  padding: 12,
                  background: 'rgba(2, 6, 23, 0.5)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border-subtle)',
                }}
              >
                <span style={{ fontSize: 11, color: 'var(--text-dim)', display: 'block' }}>Ledger Invariant</span>
                <strong
                  style={{
                    fontSize: 14,
                    display: 'flex',
                    alignItems: 'center',
                    gap: 4,
                    marginTop: 2,
                    color: report.ledger_balanced ? 'var(--emerald-400)' : '#F43F5E',
                  }}
                >
                  {report.ledger_balanced ? (
                    <>
                      <CheckCircle2 size={16} /> BALANCED
                    </>
                  ) : (
                    <>
                      <AlertTriangle size={16} /> UNBALANCED
                    </>
                  )}
                </strong>
              </div>
            </div>

            {/* Findings List */}
            {(() => {
              const discrepancies = Array.isArray(report.discrepancies) ? report.discrepancies : [];
              return (
                <div style={{ marginBottom: 16 }}>
                  <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-dim)', textTransform: 'uppercase' }}>
                    Discrepancies & Actions ({discrepancies.length})
                  </span>
                  <div
                    style={{
                      marginTop: 8,
                      maxHeight: 180,
                      overflowY: 'auto',
                      display: 'flex',
                      flexDirection: 'column',
                      gap: 6,
                    }}
                  >
                    {discrepancies.length === 0 ? (
                      <div
                        style={{
                          padding: 16,
                          textAlign: 'center',
                          background: 'rgba(2, 6, 23, 0.4)',
                          borderRadius: 'var(--radius-md)',
                          color: 'var(--emerald-400)',
                          fontSize: 12,
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          gap: 6,
                        }}
                      >
                        <ShieldCheck size={16} /> All accounts, transactions, and settlement journals are 100% synchronized!
                      </div>
                    ) : (
                      discrepancies.map((disc, idx) => (
                        <div
                          key={idx}
                          style={{
                            padding: '8px 12px',
                            background: 'rgba(2, 6, 23, 0.6)',
                            borderRadius: 6,
                            border: '1px solid var(--border-subtle)',
                            fontSize: 12,
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'space-between',
                          }}
                        >
                          <div>
                            <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-muted)' }}>
                              {disc.payment_id}
                            </span>
                            <span style={{ marginLeft: 8, color: '#F8FAFC' }}>{disc.details}</span>
                          </div>
                          <span
                            style={{
                              fontSize: 10,
                              fontWeight: 700,
                              padding: '2px 6px',
                              borderRadius: 3,
                              background: 'rgba(56, 189, 248, 0.15)',
                              color: '#38BDF8',
                              border: '1px solid rgba(56, 189, 248, 0.3)',
                            }}
                          >
                            {disc.type}
                          </span>
                        </div>
                      ))
                    )}
                  </div>
                </div>
              );
            })()}

            <div style={{ fontSize: 11, color: 'var(--text-dim)', textAlign: 'right' }}>
              Execution completed in {report.duration_ms}ms · Report ID: {report.id}
            </div>
          </div>
        )}

        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn btn-secondary" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
